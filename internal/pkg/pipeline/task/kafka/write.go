package kafka

import (
	"fmt"
	"sync"
	"time"

	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/ack"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

// write produces records to the Kafka topic using the codec selected by the format field.
func (k *kafka) write(input <-chan *record.Record) error {
	cfg, err := k.buildProducerConfig()
	if err != nil {
		return fmt.Errorf("failed to build producer config: %w", err)
	}

	p, err := ckafka.NewProducer(cfg)
	if err != nil {
		return fmt.Errorf("failed to create producer: %w", err)
	}
	defer p.Close()

	codec, err := k.newCodec()
	if err != nil {
		return err
	}

	// deliveryCh is drained by a goroutine; closed after Flush so wg.Wait() guarantees no race on firstDeliveryErr.
	deliveryCh := make(chan ckafka.Event, 100)
	var (
		wg               sync.WaitGroup
		firstDeliveryErr error
	)

	// acks for records the producer has taken but not yet confirmed. A message that
	// never produces a delivery report — a flush timeout, typically — would
	// otherwise leave its source record unsettled and the pipeline waiting on it
	// forever, so whatever is left here at the end is failed explicitly.
	var (
		pendingMu sync.Mutex
		pending   = make(map[*ack.Ack]struct{})
	)

	settle := func(a *ack.Ack, deliveryErr error) {

		if a == nil {
			return
		}

		pendingMu.Lock()
		delete(pending, a)
		pendingMu.Unlock()

		if deliveryErr != nil {
			a.Fail()
			return
		}

		a.Done()

	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for e := range deliveryCh {
			m, ok := e.(*ckafka.Message)
			if !ok {
				continue
			}
			a, _ := m.Opaque.(*ack.Ack)
			if m.TopicPartition.Error != nil {
				if firstDeliveryErr == nil {
					firstDeliveryErr = m.TopicPartition.Error
					fmt.Printf("delivery failed for topic %s partition %d: %v\n",
						k.Topic, m.TopicPartition.Partition, m.TopicPartition.Error)
				}
				// the source record never made it to the topic, so fail its
				// ack: the source must leave it unacknowledged for retry.
				settle(a, m.TopicPartition.Error)
				continue
			}
			// settle only on broker-confirmed delivery, not on local enqueue.
			settle(a, nil)
		}
	}()

	var produceErr error
	for {
		r, ok := k.GetRecord(input)
		if !ok {
			break
		}

		msgBytes, err := codec.serialize(k.Topic, r.Data)
		if err != nil {
			produceErr = fmt.Errorf("failed to serialize record for topic %s: %w", k.Topic, err)
			ack.Reject(r.Context)
			break
		}

		var opaque any
		a, tracked := ack.FromContext(r.Context)
		if tracked {
			opaque = a
			pendingMu.Lock()
			pending[a] = struct{}{}
			pendingMu.Unlock()
		}

		if err = p.Produce(&ckafka.Message{
			TopicPartition: ckafka.TopicPartition{Topic: &k.Topic, Partition: ckafka.PartitionAny},
			Value:          msgBytes,
			Opaque:         opaque,
		}, deliveryCh); err != nil {
			produceErr = fmt.Errorf("failed to enqueue message to topic %s: %w", k.Topic, err)
			// the producer never took it, so no delivery report is coming
			settle(a, produceErr)
			break
		}
	}

	// Always flush so enqueued messages get delivery reports and the goroutine exits cleanly.
	timeout := time.Duration(k.Timeout)
	remaining := p.Flush(int(timeout.Milliseconds()))

	// Close the producer BEFORE closing deliveryCh:
	p.Close()
	close(deliveryCh)
	wg.Wait()

	// no delivery report ever arrived for these, so nothing else will settle them
	pendingMu.Lock()
	for a := range pending {
		a.Fail()
	}
	pending = nil
	pendingMu.Unlock()

	if produceErr != nil {
		return produceErr
	}
	if firstDeliveryErr != nil {
		return fmt.Errorf("delivery failed for topic %s: %w", k.Topic, firstDeliveryErr)
	}
	if remaining > 0 {
		return fmt.Errorf("%d messages failed to deliver to topic %s within %s", remaining, k.Topic, timeout)
	}
	return nil
}
