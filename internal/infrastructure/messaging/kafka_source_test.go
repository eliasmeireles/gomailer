package messaging

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kfake"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/eliasmeireles/gomailer/internal/application/config"
	"github.com/eliasmeireles/gomailer/internal/core/mailer"
	"github.com/eliasmeireles/gomailer/internal/core/model"
)

const testTopic = "mailer-service"

var testKafkaPolicy = mailer.RetryPolicy{MaxAttempts: 3, BaseDelay: 200 * time.Millisecond, MaxDelay: time.Second}

type handledCall struct {
	body    string
	attempt int
	at      time.Time
}

// scriptedHandler answers each attempt with the outcome at that index (the last one repeats).
type scriptedHandler struct {
	mu       sync.Mutex
	outcomes []mailer.Outcome
	calls    []handledCall
	notify   chan handledCall
}

func newScriptedHandler(outcomes ...mailer.Outcome) *scriptedHandler {
	return &scriptedHandler{outcomes: outcomes, notify: make(chan handledCall, 10)}
}

func (h *scriptedHandler) handle(body []byte, attempt int) mailer.Outcome {
	call := handledCall{body: string(body), attempt: attempt, at: time.Now()}
	h.mu.Lock()
	h.calls = append(h.calls, call)
	h.mu.Unlock()
	h.notify <- call
	return h.outcomes[min(attempt, len(h.outcomes))-1]
}

func (h *scriptedHandler) next(t *testing.T) handledCall {
	t.Helper()
	select {
	case call := <-h.notify:
		return call
	case <-time.After(10 * time.Second):
		require.Fail(t, "handler was not called")
		return handledCall{}
	}
}

func startKafkaSource(t *testing.T, handler Handler, createTopics bool) ([]string, *KafkaSource) {
	t.Helper()
	opts := []kfake.Opt{kfake.NumBrokers(1)}
	if !createTopics {
		opts = append(opts, kfake.SeedTopics(1, testTopic, testTopic+".retry", testTopic+".dlq"))
	}
	cluster, err := kfake.NewCluster(opts...)
	require.NoError(t, err)
	t.Cleanup(cluster.Close)

	cfg := config.KafkaConfig{Brokers: cluster.ListenAddrs(), Topic: testTopic, GroupID: "test", CreateTopics: createTopics, Partitions: 1, Replication: 1}
	source := NewKafkaSource(cfg, testKafkaPolicy, handler)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- source.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	require.Eventually(t, source.Ready, 10*time.Second, 20*time.Millisecond)
	return cluster.ListenAddrs(), source
}

func produce(t *testing.T, brokers []string, topic, value string) {
	t.Helper()
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	require.NoError(t, err)
	defer client.Close()
	require.NoError(t, client.ProduceSync(context.Background(), &kgo.Record{Topic: topic, Key: []byte("k"), Value: []byte(value)}).FirstErr())
}

func readOne(t *testing.T, brokers []string, topic string) *kgo.Record {
	t.Helper()
	client, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.ConsumeTopics(topic), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	require.NoError(t, err)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for {
		fetches := client.PollFetches(ctx)
		require.NoError(t, ctx.Err(), "no record on %s", topic)
		if records := fetches.Records(); len(records) > 0 {
			return records[0]
		}
	}
}

func TestKafkaSource(t *testing.T) {
	t.Run("given a record that is sent then handle it once on attempt 1", func(t *testing.T) {
		handler := newScriptedHandler(mailer.Outcome{Action: mailer.ActionDone})
		brokers, source := startKafkaSource(t, handler.handle, false)

		produce(t, brokers, testTopic, `{"id":"k1"}`)

		call := handler.next(t)
		assert.Equal(t, `{"id":"k1"}`, call.body)
		assert.Equal(t, 1, call.attempt)
		assert.Equal(t, "kafka", source.Name())
	})

	t.Run("given a temporary failure then retry after the delay on the next attempt", func(t *testing.T) {
		handler := newScriptedHandler(
			mailer.Outcome{Action: mailer.ActionRetry, RetryIn: 300 * time.Millisecond},
			mailer.Outcome{Action: mailer.ActionDone},
		)
		brokers, _ := startKafkaSource(t, handler.handle, false)

		produce(t, brokers, testTopic, `{"id":"k2"}`)

		first := handler.next(t)
		second := handler.next(t)
		assert.Equal(t, 1, first.attempt)
		assert.Equal(t, 2, second.attempt)
		assert.Equal(t, `{"id":"k2"}`, second.body)
		assert.GreaterOrEqual(t, second.at.Sub(first.at), 290*time.Millisecond)
	})

	t.Run("given a final failure then move the record to the dead-letter topic with the failure headers", func(t *testing.T) {
		handler := newScriptedHandler(mailer.Outcome{
			Action: mailer.ActionDeadLetter,
			Event:  model.DeliveryEvent{ErrorCode: model.CodeAPIInvalidReceiver, Cause: "invalid to"},
		})
		brokers, _ := startKafkaSource(t, handler.handle, true)

		produce(t, brokers, testTopic, `{"id":"k3"}`)
		handler.next(t)

		record := readOne(t, brokers, testTopic+".dlq")
		assert.Equal(t, `{"id":"k3"}`, string(record.Value))
		assert.Equal(t, []byte("k"), record.Key)
		assert.Equal(t, "api_invalid_receiver", kafkaHeader(record.Headers, headerErrorCode))
		assert.Equal(t, "invalid to", kafkaHeader(record.Headers, headerCause))
		assert.Equal(t, 1, kafkaAttempt(record.Headers))
		assert.NotEmpty(t, kafkaHeader(record.Headers, headerFailedAt))
	})

	t.Run("given a cancelled context while connecting then stop", func(t *testing.T) {
		source := NewKafkaSource(config.KafkaConfig{Brokers: []string{"127.0.0.1:1"}, Topic: testTopic, GroupID: "g"}, testKafkaPolicy, nil)
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		defer cancel()

		err := source.Run(ctx)

		require.Error(t, err)
		assert.False(t, source.Ready())
	})
}

func TestKafkaHeaders(t *testing.T) {
	t.Run("given no headers then first attempt and due now", func(t *testing.T) {
		assert.Equal(t, 1, kafkaAttempt(nil))
		assert.True(t, kafkaNotBefore(nil).IsZero())
	})

	t.Run("given attempt and not-before headers then parse them", func(t *testing.T) {
		headers := []kgo.RecordHeader{{Key: headerAttempt, Value: []byte("3")}, {Key: headerNotBefore, Value: []byte("1790000000000")}}

		assert.Equal(t, 3, kafkaAttempt(headers))
		assert.Equal(t, time.UnixMilli(1790000000000), kafkaNotBefore(headers))
	})

	t.Run("given invalid values then fall back to defaults", func(t *testing.T) {
		headers := []kgo.RecordHeader{{Key: headerAttempt, Value: []byte("x")}, {Key: headerNotBefore, Value: []byte("soon")}}

		assert.Equal(t, 1, kafkaAttempt(headers))
		assert.True(t, kafkaNotBefore(headers).IsZero())
	})
}

func TestKafkaWaitUntilDue(t *testing.T) {
	source := NewKafkaSource(config.KafkaConfig{Topic: testTopic}, testKafkaPolicy, nil)

	t.Run("given a past time then return immediately", func(t *testing.T) {
		assert.True(t, source.waitUntilDue(context.Background(), time.Now().Add(-time.Second)))
	})

	t.Run("given a cancelled context then return false", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		assert.False(t, source.waitUntilDue(ctx, time.Now().Add(time.Hour)))
	})
}

func TestKafkaTopicsAndOptions(t *testing.T) {
	t.Run("must name the retry and dead-letter topics after the main topic", func(t *testing.T) {
		assert.Equal(t, []string{"emails", "emails.retry", "emails.dlq"}, newKafkaTopics("emails").all())
	})

	t.Run("given tls and sasl then add their options", func(t *testing.T) {
		base := len(kafkaClientOptions(config.KafkaConfig{Brokers: []string{"b:9092"}}))

		for _, mechanism := range []string{config.SASLPlain, config.SASLScramSHA256, config.SASLScramSHA512} {
			opts := kafkaClientOptions(config.KafkaConfig{Brokers: []string{"b:9092"}, TLS: true, SASLMechanism: mechanism, SASLUser: "u", SASLPass: "p"})
			assert.Len(t, opts, base+2, mechanism)
		}
	})
}
