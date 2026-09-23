package messaging

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/eliasmeireles/gomailer/internal/application/config"
	"github.com/eliasmeireles/gomailer/internal/core/mailer"
)

// rebalanceMargin is added to the longest retry wait: rebalances are blocked while a batch is
// processed (including the wait of a retry record), so the group must tolerate that long.
const rebalanceMargin = time.Minute

// KafkaSource consumes email requests from Kafka. Offsets are committed only after each record
// is resolved (sent, handed to the failure callback, moved to the retry topic or to the DLQ),
// and rebalances are blocked while a batch is in flight, so a record is never processed twice
// by two members at the same time.
type KafkaSource struct {
	cfg     config.KafkaConfig
	policy  mailer.RetryPolicy
	handler Handler
	topics  kafkaTopics
	now     func() time.Time

	producer *kgo.Client
	ready    atomic.Bool
}

// NewKafkaSource creates the Kafka source; the retry topic waits follow policy.
func NewKafkaSource(cfg config.KafkaConfig, policy mailer.RetryPolicy, handler Handler) *KafkaSource {
	return &KafkaSource{cfg: cfg, policy: policy, handler: handler, topics: newKafkaTopics(cfg.Topic), now: time.Now}
}

// Name identifies the source in logs.
func (s *KafkaSource) Name() string { return "kafka" }

// Ready reports whether the brokers are reachable and the last polls succeeded.
func (s *KafkaSource) Ready() bool { return s.ready.Load() }

// Run connects (retrying with backoff), ensures the topics when configured and consumes the
// main and retry topics until ctx is cancelled.
func (s *KafkaSource) Run(ctx context.Context) error {
	producer, err := kgo.NewClient(kafkaClientOptions(s.cfg)...)
	if err != nil {
		return fmt.Errorf("create producer: %w", err)
	}
	defer producer.Close()
	s.producer = producer

	if err := s.connect(ctx, producer); err != nil {
		return err
	}

	main, err := s.newConsumer(s.cfg.GroupID, s.topics.main)
	if err != nil {
		return err
	}
	defer main.Close()

	retry, err := s.newConsumer(s.cfg.GroupID+"-retry", s.topics.retry)
	if err != nil {
		return err
	}
	defer retry.Close()

	s.ready.Store(true)
	log.Infof("Kafka source started: topic %s, retry %s, dead-letter %s, group %s", s.topics.main, s.topics.retry, s.topics.deadLetter, s.cfg.GroupID)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, loop := range []struct {
		client  *kgo.Client
		waitDue bool
	}{{main, false}, {retry, true}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = s.consume(ctx, loop.client, loop.waitDue)
		}()
	}
	wg.Wait()
	s.ready.Store(false)

	return errors.Join(errs...)
}

// connect pings the brokers with exponential backoff and creates the topics when configured.
func (s *KafkaSource) connect(ctx context.Context, client *kgo.Client) error {
	backoff := initialBackoff
	for {
		err := client.Ping(ctx)
		if err == nil && s.cfg.CreateTopics {
			err = s.topics.ensure(ctx, kadm.NewClient(client), s.cfg.Partitions, s.cfg.Replication)
		}
		if err == nil {
			return nil
		}

		log.Warnf("Kafka connect failed: %v; retrying in %s", err, backoff)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxBackoff)
	}
}

func (s *KafkaSource) newConsumer(group, topic string) (*kgo.Client, error) {
	opts := append(kafkaClientOptions(s.cfg),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topic),
		kgo.DisableAutoCommit(),
		kgo.BlockRebalanceOnPoll(),
		kgo.RebalanceTimeout(s.policy.MaxDelay+rebalanceMargin),
	)
	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("create consumer for %s: %w", topic, err)
	}
	return client, nil
}
