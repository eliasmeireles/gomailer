package messaging

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/twmb/franz-go/pkg/kgo"
)

// consume polls client until ctx is cancelled. Each record is processed in order and only the
// records resolved so far are committed; when waitDue is set (retry topic) a record is processed
// only once its x-not-before time has passed.
func (s *KafkaSource) consume(ctx context.Context, client *kgo.Client, waitDue bool) error {
	// With BlockRebalanceOnPoll, Close waits for the rebalance to be allowed before leaving the
	// group, so it must be allowed on every exit path.
	defer client.AllowRebalance()

	for {
		fetches := client.PollFetches(ctx)
		if ctx.Err() != nil || fetches.IsClientClosed() {
			return ctx.Err()
		}

		healthy := true
		fetches.EachError(func(topic string, partition int32, err error) {
			healthy = false
			log.Warnf("Kafka fetch error on %s[%d]: %v", topic, partition, err)
		})
		s.ready.Store(healthy)

		resolved := s.processBatch(ctx, fetches, waitDue)
		if len(resolved) > 0 {
			if err := client.CommitRecords(ctx, resolved...); err != nil {
				log.Errorf("Kafka commit failed: %v", err)
			}
		}
		client.AllowRebalance()
	}
}

// processBatch handles the records in order and stops at the first one that cannot be
// resolved (context cancelled while waiting or producing), returning the resolved records.
func (s *KafkaSource) processBatch(ctx context.Context, fetches kgo.Fetches, waitDue bool) []*kgo.Record {
	var resolved []*kgo.Record
	stopped := false
	fetches.EachRecord(func(record *kgo.Record) {
		if stopped {
			return
		}
		if waitDue && !s.waitUntilDue(ctx, kafkaNotBefore(record.Headers)) {
			stopped = true
			return
		}

		attempt := kafkaAttempt(record.Headers)
		if err := s.apply(ctx, record, attempt, s.handler(record.Value, attempt)); err != nil {
			log.Errorf("Kafka record %s[%d]@%d not resolved: %v", record.Topic, record.Partition, record.Offset, err)
			stopped = true
			return
		}
		resolved = append(resolved, record)
	})
	return resolved
}

// waitUntilDue blocks until due; it returns false when ctx is cancelled first.
func (s *KafkaSource) waitUntilDue(ctx context.Context, due time.Time) bool {
	wait := due.Sub(s.now())
	if wait <= 0 {
		return true
	}

	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
