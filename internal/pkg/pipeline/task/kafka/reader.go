package kafka

import (
	"fmt"
	"sync"
	"time"

	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
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
