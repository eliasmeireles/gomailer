package kafka

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/eliasmeireles/gomailer/client"
)

func email() client.Email {
	return client.Email{ID: "k1", From: "no-reply@exemplo.com.br", To: []string{"maria@exemplo.com.br"}, Subject: "Oi", HTML: "<p>Oi</p>"}
}

func TestSender(t *testing.T) {
	t.Run("given an email then produce it keyed by id in the wire format", func(t *testing.T) {
		cluster, err := kfake.NewCluster(kfake.NumBrokers(1), kfake.SeedTopics(1, "mailer-service"))
		require.NoError(t, err)
		defer cluster.Close()
		sender, err := New(Config{Brokers: cluster.ListenAddrs()})
		require.NoError(t, err)
		defer sender.Close()

		require.NoError(t, sender.HealthCheck(context.Background()))
		require.NoError(t, sender.Send(context.Background(), email()))

		reader, err := kgo.NewClient(kgo.SeedBrokers(cluster.ListenAddrs()...), kgo.ConsumeTopics("mailer-service"), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
		require.NoError(t, err)
		defer reader.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		records := reader.PollFetches(ctx).Records()
		require.Len(t, records, 1)
		assert.Equal(t, "k1", string(records[0].Key))
		var wire map[string]any
		require.NoError(t, json.Unmarshal(records[0].Value, &wire))
		assert.Equal(t, []any{"maria@exemplo.com.br"}, wire["receiver"])
		assert.Equal(t, "PHA+T2k8L3A+", wire["body"])
	})

	t.Run("given an invalid email then fail before producing", func(t *testing.T) {
		sender, err := New(Config{Brokers: []string{"127.0.0.1:1"}})
		require.NoError(t, err)

		require.ErrorIs(t, sender.Send(context.Background(), client.Email{}), client.ErrInvalidEmail)
	})

	t.Run("given unreachable brokers then fail the health check", func(t *testing.T) {
		sender, err := New(Config{Brokers: []string{"127.0.0.1:1"}, Timeout: 300 * time.Millisecond})
		require.NoError(t, err)

		require.Error(t, sender.HealthCheck(context.Background()))
	})

	t.Run("given no brokers or an unknown sasl mechanism then refuse to build", func(t *testing.T) {
		_, err := New(Config{})
		require.Error(t, err)

		_, err = New(Config{Brokers: []string{"b:9092"}, SASLMechanism: "oauth"})
		require.Error(t, err)
	})

	t.Run("given tls and every sasl mechanism then build", func(t *testing.T) {
		for _, mechanism := range []string{SASLPlain, SASLScramSHA256, SASLScramSHA512} {
			sender, err := New(Config{Brokers: []string{"b:9092"}, TLS: true, SASLMechanism: mechanism, SASLUser: "u", SASLPass: "p"})
			require.NoError(t, err, mechanism)
			require.NoError(t, sender.Close())
		}
	})
}
