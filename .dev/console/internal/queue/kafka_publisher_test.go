package queue

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/eliasmeireles/gomailer/dev/console/internal/message"
)

func TestKafkaPublisher(t *testing.T) {
	t.Run("given an email then produce it as json keyed by id", func(t *testing.T) {
		cluster, err := kfake.NewCluster(kfake.NumBrokers(1), kfake.SeedTopics(1, "mailer-service"))
		require.NoError(t, err)
		defer cluster.Close()
		publisher, err := NewKafkaPublisher(cluster.ListenAddrs(), "mailer-service")
		require.NoError(t, err)

		require.NoError(t, publisher.Publish(context.Background(), message.Email{ID: "k1", Subject: "Hi"}))

		reader, err := kgo.NewClient(kgo.SeedBrokers(cluster.ListenAddrs()...), kgo.ConsumeTopics("mailer-service"), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
		require.NoError(t, err)
		defer reader.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		records := reader.PollFetches(ctx).Records()
		require.Len(t, records, 1)
		assert.Equal(t, "k1", string(records[0].Key))
		var email message.Email
		require.NoError(t, json.Unmarshal(records[0].Value, &email))
		assert.Equal(t, "Hi", email.Subject)
	})

	t.Run("given unreachable brokers then return an error", func(t *testing.T) {
		publisher, err := NewKafkaPublisher([]string{"127.0.0.1:1"}, "mailer-service")
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		err = publisher.Publish(ctx, message.Email{ID: "k1"})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "produce to kafka")
	})
}
