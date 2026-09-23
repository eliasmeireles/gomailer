package monitor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"
)

const testKafkaTopic = "mailer-service"

func newKafkaCluster(t *testing.T) []string {
	t.Helper()
	cluster, err := kfake.NewCluster(kfake.NumBrokers(1), kfake.SeedTopics(1, testKafkaTopic, testKafkaTopic+".retry", testKafkaTopic+".dlq"))
	require.NoError(t, err)
	t.Cleanup(cluster.Close)
	return cluster.ListenAddrs()
}

func produceRecords(t *testing.T, brokers []string, records ...*kgo.Record) {
	t.Helper()
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	require.NoError(t, err)
	defer client.Close()
	require.NoError(t, client.ProduceSync(context.Background(), records...).FirstErr())
}

func deadLetterRecord(key, code string) *kgo.Record {
	return &kgo.Record{Topic: testKafkaTopic + ".dlq", Key: []byte(key), Value: []byte(`{}`), Headers: []kgo.RecordHeader{
		{Key: "x-attempt", Value: []byte("3")},
		{Key: "x-error-code", Value: []byte(code)},
		{Key: "x-error-cause", Value: []byte("cause of " + key)},
		{Key: "x-failed-at", Value: []byte("2026-09-23T12:00:00Z")},
	}}
}

func TestKafkaMonitor(t *testing.T) {
	t.Run("given pending, retrying and dead-lettered records then count them", func(t *testing.T) {
		brokers := newKafkaCluster(t)
		produceRecords(t, brokers,
			&kgo.Record{Topic: testKafkaTopic, Value: []byte("a")},
			&kgo.Record{Topic: testKafkaTopic, Value: []byte("b")},
			&kgo.Record{Topic: testKafkaTopic + ".retry", Value: []byte("c")},
			deadLetterRecord("d1", "api_rate_limited"),
		)
		kafka, err := NewKafka(brokers, testKafkaTopic, "gomailer")
		require.NoError(t, err)

		stats, err := kafka.Stats(context.Background())

		require.NoError(t, err)
		assert.Equal(t, QueueStats{Messages: 2, Retrying: 1, DeadLettered: 1}, stats)
	})

	t.Run("given dead letters then peek them with their failure headers", func(t *testing.T) {
		brokers := newKafkaCluster(t)
		produceRecords(t, brokers, deadLetterRecord("d1", "api_rate_limited"), deadLetterRecord("d2", "api_invalid_receiver"))
		kafka, err := NewKafka(brokers, testKafkaTopic, "gomailer")
		require.NoError(t, err)

		letters, err := kafka.PeekDeadLetters(context.Background())

		require.NoError(t, err)
		require.Len(t, letters, 2)
		assert.Equal(t, DeadLetter{
			Source: "kafka", MessageID: "d1", Attempt: 3, ErrorCode: "api_rate_limited", Cause: "cause of d1",
			FailedAt: time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC), Payload: "{}",
		}, letters[0])
		assert.Equal(t, "api_invalid_receiver", letters[1].ErrorCode)
	})

	t.Run("given an empty dead-letter topic then peek nothing", func(t *testing.T) {
		kafka, err := NewKafka(newKafkaCluster(t), testKafkaTopic, "gomailer")
		require.NoError(t, err)

		letters, err := kafka.PeekDeadLetters(context.Background())

		require.NoError(t, err)
		assert.Empty(t, letters)
	})

	t.Run("given purge then the dead-letter topic becomes empty", func(t *testing.T) {
		brokers := newKafkaCluster(t)
		produceRecords(t, brokers, deadLetterRecord("d1", "api_rate_limited"))
		kafka, err := NewKafka(brokers, testKafkaTopic, "gomailer")
		require.NoError(t, err)

		require.NoError(t, kafka.PurgeDeadLetters(context.Background()))
		stats, err := kafka.Stats(context.Background())

		require.NoError(t, err)
		assert.Zero(t, stats.DeadLettered)
	})

	t.Run("given unreachable brokers then return error", func(t *testing.T) {
		kafka, err := NewKafka([]string{"127.0.0.1:1"}, testKafkaTopic, "gomailer")
		require.NoError(t, err)

		_, err = kafka.Stats(context.Background())

		require.Error(t, err)
	})
}
