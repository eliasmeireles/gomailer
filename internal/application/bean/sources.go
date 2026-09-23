package bean

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/eliasmeireles/gomailer/internal/adapter/consumer"
	"github.com/eliasmeireles/gomailer/internal/adapter/httpapi"
	"github.com/eliasmeireles/gomailer/internal/application/config"
	"github.com/eliasmeireles/gomailer/internal/application/runner"
	"github.com/eliasmeireles/gomailer/internal/core/model"
	"github.com/eliasmeireles/gomailer/internal/infrastructure/messaging"
)

// InitSources builds the sources enabled by MAILER_SOURCES. Must run after InitMailer.
func InitSources() []runner.Source {
	names, err := config.Sources()
	if err != nil {
		log.Fatalf("Invalid sources: %v", err)
	}

	sources := make([]runner.Source, 0, len(names))
	for _, name := range names {
		source, err := newSource(name)
		if err != nil {
			log.Fatalf("Failed to initialize source %s: %v", name, err)
		}
		sources = append(sources, source)
	}
	return sources
}

func newSource(name config.SourceName) (runner.Source, error) {
	switch name {
	case config.SourceRabbitMQ:
		handler := consumer.NewMailerConsumer(MailerService).Handle
		return messaging.NewSource(messaging.NewConsumer(messaging.NewRabbitMQConfig(), RetryPolicy), handler), nil
	case config.SourceKafka:
		cfg, err := config.NewKafkaConfig()
		if err != nil {
			return nil, err
		}
		return messaging.NewKafkaSource(cfg, RetryPolicy, consumer.NewMailerConsumer(MailerService).Handle), nil
	case config.SourceHTTP:
		cfg, err := config.NewHTTPAPIConfig()
		if err != nil {
			return nil, err
		}
		return httpapi.NewServer(cfg, MailerService, model.NewID), nil
	default:
		return nil, fmt.Errorf("unsupported source %q", name)
	}
}
