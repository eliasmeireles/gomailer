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
	envHTTPAPIDisabled   = "HTTP_API_DISABLED"
)

var knownSources = []SourceName{SourceRabbitMQ, SourceHTTP, SourceKafka}

// Sources returns the enabled sources: those listed in MAILER_SOURCES (comma-separated, default
// "rabbitmq") plus the HTTP source, which is always enabled unless HTTP_API_DISABLED is true.
// Listing "http" alone runs the HTTP source only.
//
// Example:
//
//	MAILER_SOURCES=rabbitmq,kafka       # rabbitmq, kafka and http
//	MAILER_SOURCES=rabbitmq HTTP_API_DISABLED=true  # rabbitmq only
func Sources() ([]SourceName, error) {
	sources, err := listedSources()
	if err != nil {
		return nil, err
	}

	httpListed := slices.Contains(sources, SourceHTTP)
	if !isTrue(envOrDefault(envHTTPAPIDisabled, "false")) {
		if !httpListed {
			sources = append(sources, SourceHTTP)
		}
		return sources, nil
	}

	if httpListed {
		return nil, fmt.Errorf("%s lists %q but %s is true", envMailerSources, SourceHTTP, envHTTPAPIDisabled)
	}
	return sources, nil
}

func listedSources() ([]SourceName, error) {
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
