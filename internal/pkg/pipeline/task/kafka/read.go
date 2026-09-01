package kafka

import (
	"context"
	"fmt"
	"time"

	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"

	"github.com/patterninc/caterpillar/internal/pkg/pipeline/ack"
	"github.com/patterninc/caterpillar/internal/pkg/pipeline/record"
)

// read polls messages from the topic, standalone mode reads from beginning on every run, group mode resumes from committed offsets.
func (k *kafka) read(ctx context.Context, output chan<- *record.Record) error {
	if k.EndAfter > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(k.EndAfter))
		defer cancel()
	}

	standalone := k.GroupID == ""

	var cfg *ckafka.ConfigMap
	var err error
	if standalone {
		cfg, err = k.buildStandaloneConsumerConfig()
	} else {
		cfg, err = k.buildConsumerConfig()
	}
	if err != nil {
		return fmt.Errorf("failed to build consumer config: %w", err)
	}

	c, err := ckafka.NewConsumer(cfg)
	if err != nil {
		return fmt.Errorf("failed to create consumer: %w", err)
	}

	var r *reader
	if !standalone {
		k.ensureReadTracker()
		r = &reader{
			k:        k,
			consumer: c,
			offsets:  newOffsetTracker(),
		}
		k.registerReader(r)
	} else {
		// standalone never stores offsets, so the consumer need not outlive Run
		defer func() {
			if err := c.Close(); err != nil {
				fmt.Printf("warning: error closing kafka consumer: %v\n", err)
			}
		}()
	}

	if standalone {
		fmt.Printf("no group_id set — standalone read from beginning of topic %s\n", k.Topic)
		if err := k.assignAllPartitions(c); err != nil {
			return fmt.Errorf("failed to assign partitions: %w", err)
		}
	} else if err := c.SubscribeTopics([]string{k.Topic}, nil); err != nil {
		return fmt.Errorf("failed to subscribe to topic %s: %w", k.Topic, err)
	}

	codec, err := k.newCodec()
	if err != nil {
		return err
	}

	timeout := time.Duration(k.Timeout)
	retriesNumber := 0
	recordsRead := 0
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("kafka end_after duration reached for topic %s, stopping reader\n", k.Topic)
			return nil
		default:
		}

		if r != nil {
			if paused, err := r.allAssignedPaused(); err != nil {
				fmt.Printf("warning: failed to check kafka assignment for topic %s: %v\n", k.Topic, err)
			} else if paused {
				fmt.Printf("all assigned partitions paused for topic %s, stopping reader\n", k.Topic)
				return nil
			}
		}

		var msg *ckafka.Message
		if r != nil {
			msg, err = r.readMessage(timeout)
		} else {
			msg, err = c.ReadMessage(timeout)
		}
		if err != nil {
			if kafkaErr, ok := err.(ckafka.Error); ok && kafkaErr.Code() == ckafka.ErrTimedOut {
				retriesNumber++
				fmt.Printf("kafka read timeout for attempt #%d on topic %s\n", retriesNumber, k.Topic)
			} else if !k.shouldRetry(err) {
				return err
			} else {
				retriesNumber++
				fmt.Printf("kafka error reading message attempt #%d: %v\n", retriesNumber, err)
			}

			if retriesNumber > *k.RetryLimit {
				fmt.Printf("kafka error while reading message, reached retry limit (%d), stopping reader\n", *k.RetryLimit)
				return nil
			}
			continue
		}
		retriesNumber = 0

		data, err := codec.deserialize(k.Topic, msg.Value)
		if err != nil {
			return fmt.Errorf("failed to deserialize message from topic %s: %w", k.Topic, err)
		}

		if standalone {
			k.SendData(ctx, data, output)
		} else if output == nil {
			// no downstream branch to settle: Track would hang Finish on Wait
			r.offsets.observe(msg.TopicPartition.Partition, int64(msg.TopicPartition.Offset))
			if err := r.storeOffset(msg.TopicPartition.Partition, int64(msg.TopicPartition.Offset)+1); err != nil {
				fmt.Printf("warning: failed to store offset for topic %s partition %d: %v\n",
					k.Topic, msg.TopicPartition.Partition, err)
			}
		} else {
			r.offsets.observe(msg.TopicPartition.Partition, int64(msg.TopicPartition.Offset))
			msgAck := ack.New()
			k.SendData(ack.WithContext(ctx, msgAck), data, output)
			k.tracker.Track(msgAck, &messageAck{
				reader:    r,
				partition: msg.TopicPartition.Partition,
				offset:    int64(msg.TopicPartition.Offset),
			})
		}

		recordsRead++
		if k.MaxRecords > 0 && recordsRead >= k.MaxRecords {
			fmt.Printf("kafka max_records (%d) reached for topic %s, stopping reader\n", k.MaxRecords, k.Topic)
			return nil
		}
	}
}

// assignAllPartitions assigns all topic partitions at OffsetBeginning, bypassing the consumer group protocol.
func (k *kafka) assignAllPartitions(c *ckafka.Consumer) error {
	initTimeoutMs := int(time.Duration(defaultTimeout).Milliseconds())
	meta, err := c.GetMetadata(&k.Topic, false, initTimeoutMs)
	if err != nil {
		return fmt.Errorf("failed to get metadata for topic %s: %w", k.Topic, err)
	}

	topicMeta, ok := meta.Topics[k.Topic]
	if !ok || len(topicMeta.Partitions) == 0 {
		return fmt.Errorf("topic %s not found or has no partitions", k.Topic)
	}

	partitions := make([]ckafka.TopicPartition, len(topicMeta.Partitions))
	for i, p := range topicMeta.Partitions {
		partitions[i] = ckafka.TopicPartition{
			Topic:     &k.Topic,
			Partition: p.ID,
			Offset:    ckafka.OffsetBeginning,
		}
	}
	return c.Assign(partitions)
}

func (k *kafka) shouldRetry(err error) bool {
	if kafkaErr, ok := err.(ckafka.Error); ok {
		switch kafkaErr.Code() {
		case ckafka.ErrUnknownTopicOrPart, ckafka.ErrTopicException,
			ckafka.ErrGroupAuthorizationFailed, ckafka.ErrTopicAuthorizationFailed:
			return false
		}
	}
	return true
}
