package messaging

import (
	"context"
	"errors"
	"fmt"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
)

// kafkaTopics are the topics used by the Kafka source:
//
//	<topic>        main topic, consumed by <group>
//	<topic>.retry  temporary failures, consumed by <group>-retry once their x-not-before is due
//	<topic>.dlq    final failures nobody handled
type kafkaTopics struct {
	main       string
	retry      string
	deadLetter string
}

func newKafkaTopics(topic string) kafkaTopics {
	return kafkaTopics{main: topic, retry: topic + ".retry", deadLetter: topic + ".dlq"}
}

func (t kafkaTopics) all() []string {
	return []string{t.main, t.retry, t.deadLetter}
}

// ensure creates the missing topics; topics that already exist are left untouched.
func (t kafkaTopics) ensure(ctx context.Context, admin *kadm.Client, partitions int32, replication int16) error {
	responses, err := admin.CreateTopics(ctx, partitions, replication, nil, t.all()...)
	if err != nil {
		return fmt.Errorf("create topics: %w", err)
	}
	for _, response := range responses.Sorted() {
		if response.Err != nil && !errors.Is(response.Err, kerr.TopicAlreadyExists) {
			return fmt.Errorf("create topic %s: %w", response.Topic, response.Err)
		}
	}
	return nil
}
