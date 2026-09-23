package monitor

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
)

const kafkaPeekTimeout = 3 * time.Second

// Kafka inspects the mailer Kafka topics: consumer lag, pending retries and dead letters.
type Kafka struct {
	brokers []string
	admin   *kadm.Client
	topic   string
	group   string
}

// NewKafka creates an inspector for topic (with <topic>.retry and <topic>.dlq) consumed by group.
func NewKafka(brokers []string, topic, group string) (*Kafka, error) {
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		return nil, fmt.Errorf("create kafka client: %w", err)
	}
	return &Kafka{brokers: brokers, admin: kadm.NewClient(client), topic: topic, group: group}, nil
}

// Stats reports the records the main group still has to consume as Messages, the records the
// retry group still has to consume as Retrying, the records kept in the dead-letter topic as
// DeadLettered and the members of the main group as Consumers.
func (k *Kafka) Stats(ctx context.Context) (QueueStats, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	pending, err := k.pending(ctx, k.group, k.topic)
	if err != nil {
		return QueueStats{}, err
	}
	retrying, err := k.pending(ctx, k.group+"-retry", k.topic+".retry")
	if err != nil {
		return QueueStats{}, err
	}
	deadLettered, err := k.recordCount(ctx, k.deadLetterTopic())
	if err != nil {
		return QueueStats{}, err
	}
	groups, err := k.admin.DescribeGroups(ctx, k.group)
	if err != nil {
		return QueueStats{}, fmt.Errorf("describe kafka group %s: %w", k.group, err)
	}

	return QueueStats{Messages: pending, Consumers: len(groups[k.group].Members), Retrying: retrying, DeadLettered: deadLettered}, nil
}

// pending counts the records of topic not yet committed by group; partitions without a commit
// count from the start, like the mailer consumers (which reset to the earliest offset).
func (k *Kafka) pending(ctx context.Context, group, topic string) (int, error) {
	starts, ends, err := k.offsets(ctx, topic)
	if err != nil {
		return 0, err
	}
	committed, err := k.admin.FetchOffsets(ctx, group)
	if err != nil && !errors.Is(err, kerr.GroupIDNotFound) {
		return 0, fmt.Errorf("kafka committed offsets of %s: %w", group, err)
	}

	total := 0
	ends.Each(func(end kadm.ListedOffset) {
		from := starts[end.Topic][end.Partition].Offset
		if commit, ok := committed.Lookup(end.Topic, end.Partition); ok && commit.At >= 0 {
			from = max(from, commit.At)
		}
		total += int(max(end.Offset-from, 0))
	})
	return total, nil
}

// PeekDeadLetters returns the latest records of the dead-letter topic (up to 10 per partition).
func (k *Kafka) PeekDeadLetters(ctx context.Context) ([]DeadLetter, error) {
	ctx, cancel := context.WithTimeout(ctx, kafkaPeekTimeout)
	defer cancel()

	starts, ends, err := k.offsets(ctx, k.deadLetterTopic())
	if err != nil {
		return nil, err
	}

	partitions := map[int32]kgo.Offset{}
	expected := 0
	ends.Each(func(end kadm.ListedOffset) {
		start := max(end.Offset-deadLetterPeekLimit, starts[end.Topic][end.Partition].Offset)
		if end.Offset > start {
			partitions[end.Partition] = kgo.NewOffset().At(start)
			expected += int(end.Offset - start)
		}
	})
	if expected == 0 {
		return nil, nil
	}
	return k.read(ctx, partitions, expected)
}

// PurgeDeadLetters deletes every record of the dead-letter topic.
func (k *Kafka) PurgeDeadLetters(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	_, ends, err := k.offsets(ctx, k.deadLetterTopic())
	if err != nil {
		return err
	}
	if _, err := k.admin.DeleteRecords(ctx, ends.Offsets()); err != nil {
		return fmt.Errorf("purge kafka dead letters: %w", err)
	}
	return nil
}

func (k *Kafka) read(ctx context.Context, partitions map[int32]kgo.Offset, expected int) ([]DeadLetter, error) {
	client, err := kgo.NewClient(kgo.SeedBrokers(k.brokers...), kgo.ConsumePartitions(map[string]map[int32]kgo.Offset{k.deadLetterTopic(): partitions}))
	if err != nil {
		return nil, fmt.Errorf("create kafka reader: %w", err)
	}
	defer client.Close()

	var letters []DeadLetter
	for len(letters) < expected && ctx.Err() == nil {
		client.PollFetches(ctx).EachRecord(func(record *kgo.Record) {
			letters = append(letters, deadLetterFromRecord(record))
		})
	}
	return letters, nil
}

func (k *Kafka) recordCount(ctx context.Context, topic string) (int, error) {
	starts, ends, err := k.offsets(ctx, topic)
	if err != nil {
		return 0, err
	}
	total := 0
	ends.Each(func(end kadm.ListedOffset) {
		total += int(end.Offset - starts[end.Topic][end.Partition].Offset)
	})
	return total, nil
}

func (k *Kafka) offsets(ctx context.Context, topic string) (kadm.ListedOffsets, kadm.ListedOffsets, error) {
	starts, err := k.admin.ListStartOffsets(ctx, topic)
	if err != nil {
		return nil, nil, fmt.Errorf("kafka start offsets of %s: %w", topic, err)
	}
	ends, err := k.admin.ListEndOffsets(ctx, topic)
	if err != nil {
		return nil, nil, fmt.Errorf("kafka end offsets of %s: %w", topic, err)
	}
	if err := ends.Error(); err != nil {
		return nil, nil, fmt.Errorf("kafka offsets of %s: %w", topic, err)
	}
	return starts, ends, nil
}

func (k *Kafka) deadLetterTopic() string {
	return k.topic + ".dlq"
}

func deadLetterFromRecord(record *kgo.Record) DeadLetter {
	headers := map[string]string{}
	for _, header := range record.Headers {
		headers[header.Key] = string(header.Value)
	}
	attempt, _ := strconv.Atoi(headers["x-attempt"])
	failedAt, _ := time.Parse(time.RFC3339, headers["x-failed-at"])
	return DeadLetter{
		Source:    "kafka",
		MessageID: string(record.Key),
		Attempt:   attempt,
		ErrorCode: headers["x-error-code"],
		Cause:     headers["x-error-cause"],
		FailedAt:  failedAt,
		Payload:   string(record.Value),
	}
}
