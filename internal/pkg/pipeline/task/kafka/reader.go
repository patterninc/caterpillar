package kafka

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/ack"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

// one consumer per worker: sharing one would race under task_concurrency > 1.
type reader struct {
	k          *kafka
	consumer   *ckafka.Consumer
	offsets    *offsetTracker
	consumerMu sync.Mutex
}

type messageAck struct {
	reader    *reader
	partition int32
	offset    int64
}

func (k *kafka) newGroupReader(c *ckafka.Consumer) *reader {
	k.ensureReadTracker()
	r := &reader{
		k:        k,
		consumer: c,
		offsets:  newOffsetTracker(),
	}
	k.registerReader(r)
	return r
}

func (k *kafka) ensureReadTracker() {
	k.readersMu.Lock()
	defer k.readersMu.Unlock()
	if k.tracker == nil {
		// StoreOffsets is local; the watermark already serializes
		k.tracker = ack.NewTracker(1)
	}
}

func (k *kafka) registerReader(r *reader) {
	k.readersMu.Lock()
	defer k.readersMu.Unlock()
	k.readers = append(k.readers, r)
}

// Finish, not Run, waits for deferred stores: a join that emits on input close
// cannot settle until this task's output channel is closed, which happens only
// after Run returns.
func (k *kafka) Finish() error {
	if k.tracker != nil {
		k.tracker.Wait()
	}

	k.readersMu.Lock()
	readers := slices.Clone(k.readers)
	k.readers = nil
	k.readersMu.Unlock()

	var capped []int32
	for _, r := range readers {
		for partition, offset := range r.offsets.commitPositions() {
			if err := r.storeOffset(partition, offset); err != nil {
				fmt.Printf("warning: failed to store offset for topic %s partition %d: %v\n",
					k.Topic, partition, err)
			}
		}

		capped = append(capped, r.offsets.cappedPartitions()...)

		if err := r.commit(); err != nil {
			fmt.Printf("warning: failed to commit offsets for topic %s: %v\n", k.Topic, err)
		}
		if err := r.close(); err != nil {
			fmt.Printf("warning: error closing kafka consumer: %v\n", err)
		}
	}

	if len(capped) > 0 {
		slices.Sort(capped)
		return fmt.Errorf("kafka reader paused partitions with failed records: %v", capped)
	}

	return nil
}

func (r *reader) handleRecord(ctx context.Context, data []byte, output chan<- *record.Record, msg *ckafka.Message) {
	partition := msg.TopicPartition.Partition
	offset := int64(msg.TopicPartition.Offset)
	r.offsets.observe(partition, offset)

	if output == nil {
		// no downstream branch to settle: Track would hang Finish on Wait
		if err := r.storeOffset(partition, offset+1); err != nil {
			fmt.Printf("warning: failed to store offset for topic %s partition %d: %v\n",
				r.k.Topic, partition, err)
		}
		return
	}

	msgAck := ack.New()
	r.k.SendData(ack.WithContext(ctx, msgAck), data, output)
	r.k.tracker.Track(msgAck, &messageAck{
		reader:    r,
		partition: partition,
		offset:    offset,
	})
}

// Pause on failure so later offsets in this partition stay uncommitted and are
// re-read, rather than advancing past the gap.
func (m *messageAck) Ack(failed bool) {
	if m.reader == nil {
		return
	}

	commitTo, shouldStore, pause := m.reader.offsets.settle(m.partition, m.offset, failed)
	if shouldStore {
		if err := m.reader.storeOffset(m.partition, commitTo); err != nil {
			fmt.Printf("warning: failed to store offset for topic %s partition %d: %v\n",
				m.reader.k.Topic, m.partition, err)
		}
	}

	if pause {
		if err := m.reader.pausePartition(m.partition); err != nil {
			fmt.Printf("warning: failed to pause topic %s partition %d: %v\n",
				m.reader.k.Topic, m.partition, err)
		}
	}
}

func (r *reader) readMessage(timeout time.Duration) (*ckafka.Message, error) {
	r.consumerMu.Lock()
	defer r.consumerMu.Unlock()
	return r.consumer.ReadMessage(timeout)
}

func (r *reader) storeOffset(partition int32, offset int64) error {
	r.consumerMu.Lock()
	defer r.consumerMu.Unlock()
	topic := r.k.Topic
	_, err := r.consumer.StoreOffsets([]ckafka.TopicPartition{{
		Topic:     &topic,
		Partition: partition,
		Offset:    ckafka.Offset(offset),
	}})
	return err
}

func (r *reader) pausePartition(partition int32) error {
	r.consumerMu.Lock()
	defer r.consumerMu.Unlock()
	topic := r.k.Topic
	return r.consumer.Pause([]ckafka.TopicPartition{{
		Topic:     &topic,
		Partition: partition,
	}})
}

func (r *reader) commit() error {
	r.consumerMu.Lock()
	defer r.consumerMu.Unlock()
	_, err := r.consumer.Commit()
	return err
}

func (r *reader) close() error {
	r.consumerMu.Lock()
	defer r.consumerMu.Unlock()
	return r.consumer.Close()
}

func (r *reader) allAssignedPaused() (bool, error) {
	assignment, err := r.assignment()
	if err != nil {
		return false, err
	}

	partitions := make([]int32, 0, len(assignment))
	for _, tp := range assignment {
		partitions = append(partitions, tp.Partition)
	}

	return r.offsets.allAssignedPaused(partitions), nil
}

func (r *reader) assignment() ([]ckafka.TopicPartition, error) {
	r.consumerMu.Lock()
	defer r.consumerMu.Unlock()
	return r.consumer.Assignment()
}
