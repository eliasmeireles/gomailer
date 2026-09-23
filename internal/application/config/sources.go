package config

import (
	"fmt"
	"slices"
	"strings"
)

// SourceName identifies an inbound channel for email requests.
type SourceName string

const (
	// SourceRabbitMQ consumes requests from a RabbitMQ queue.
	SourceRabbitMQ SourceName = "rabbitmq"
	// SourceHTTP accepts requests on the POST /v1/emails endpoint.
	SourceHTTP SourceName = "http"
	// SourceKafka consumes requests from a Kafka topic.
	SourceKafka SourceName = "kafka"

	envMailerSources     = "MAILER_SOURCES"
	defaultMailerSources = "rabbitmq"
)

var knownSources = []SourceName{SourceRabbitMQ, SourceHTTP, SourceKafka}

// Sources reads MAILER_SOURCES, a comma-separated list of enabled sources (default "rabbitmq").
//
// Example:
//
//	MAILER_SOURCES=rabbitmq,http
func Sources() ([]SourceName, error) {
	var sources []SourceName
	for _, item := range strings.Split(envOrDefault(envMailerSources, defaultMailerSources), ",") {
		name := SourceName(strings.ToLower(strings.TrimSpace(item)))
		if name == "" || slices.Contains(sources, name) {
			continue
		}
		if !slices.Contains(knownSources, name) {
			return nil, fmt.Errorf("%s: unknown source %q, available: %s", envMailerSources, name, joinSources(knownSources))
		}
		sources = append(sources, name)
	}

	if len(sources) == 0 {
		return nil, fmt.Errorf("%s must enable at least one source (%s)", envMailerSources, joinSources(knownSources))
	}
	return sources, nil
}

func joinSources(sources []SourceName) string {
	names := make([]string, len(sources))
	for i, source := range sources {
		names[i] = string(source)
	}
	return strings.Join(names, ", ")
}
