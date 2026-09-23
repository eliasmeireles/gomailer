package messaging

import (
	"context"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/eliasmeireles/gomailer/internal/core/mailer"
)

// apply produces retries (with the next attempt and x-not-before) and dead letters (with the
// failure headers) before the record is committed. ProduceSync retries transient broker errors
// until ctx is cancelled, so an error here means the record must not be committed.
func (s *KafkaSource) apply(ctx context.Context, record *kgo.Record, attempt int, outcome mailer.Outcome) error {
	var out *kgo.Record
	switch outcome.Action {
	case mailer.ActionRetry:
		notBefore := s.now().Add(outcome.RetryIn)
		out = s.copyRecord(record, s.topics.retry, []kgo.RecordHeader{
			{Key: headerAttempt, Value: []byte(strconv.Itoa(attempt + 1))},
			{Key: headerNotBefore, Value: []byte(strconv.FormatInt(notBefore.UnixMilli(), 10))},
		})
	case mailer.ActionDeadLetter:
		out = s.copyRecord(record, s.topics.deadLetter, []kgo.RecordHeader{
			{Key: headerAttempt, Value: []byte(strconv.Itoa(attempt))},
			{Key: headerErrorCode, Value: []byte(outcome.Event.ErrorCode)},
			{Key: headerCause, Value: []byte(truncate(outcome.Event.Cause, maxCauseHeaderChars))},
			{Key: headerFailedAt, Value: []byte(s.now().UTC().Format(time.RFC3339))},
		})
	default:
		return nil
	}

	if err := s.producer.ProduceSync(ctx, out).FirstErr(); err != nil {
		return err
	}
	log.Infof("Kafka record %s[%d]@%d moved to %s (%s)", record.Topic, record.Partition, record.Offset, out.Topic, outcome.Action)
	return nil
}

// copyRecord keeps the key (same partition affinity) and the value, with the given headers.
func (s *KafkaSource) copyRecord(record *kgo.Record, topic string, headers []kgo.RecordHeader) *kgo.Record {
	return &kgo.Record{Topic: topic, Key: record.Key, Value: record.Value, Headers: headers}
}

// kafkaAttempt reads x-attempt; records without it are on attempt 1.
func kafkaAttempt(headers []kgo.RecordHeader) int {
	value, err := strconv.Atoi(kafkaHeader(headers, headerAttempt))
	if err != nil {
		return 1
	}
	return max(value, 1)
}

// kafkaNotBefore reads x-not-before; records without it are due immediately.
func kafkaNotBefore(headers []kgo.RecordHeader) time.Time {
	millis, err := strconv.ParseInt(kafkaHeader(headers, headerNotBefore), 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.UnixMilli(millis)
}

func kafkaHeader(headers []kgo.RecordHeader, key string) string {
	for _, header := range headers {
		if header.Key == key {
			return string(header.Value)
		}
	}
	return ""
}
