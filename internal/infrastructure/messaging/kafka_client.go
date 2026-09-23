package messaging

import (
	"crypto/tls"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"

	"github.com/eliasmeireles/gomailer/internal/application/config"
)

// kafkaClientOptions returns the connection options (brokers, TLS, SASL) shared by the
// producer, the consumers and the admin client.
func kafkaClientOptions(cfg config.KafkaConfig) []kgo.Opt {
	opts := []kgo.Opt{kgo.SeedBrokers(cfg.Brokers...)}
	if cfg.TLS {
		opts = append(opts, kgo.DialTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}))
	}

	switch cfg.SASLMechanism {
	case config.SASLPlain:
		opts = append(opts, kgo.SASL(plain.Auth{User: cfg.SASLUser, Pass: cfg.SASLPass}.AsMechanism()))
	case config.SASLScramSHA256:
		opts = append(opts, kgo.SASL(scram.Auth{User: cfg.SASLUser, Pass: cfg.SASLPass}.AsSha256Mechanism()))
	case config.SASLScramSHA512:
		opts = append(opts, kgo.SASL(scram.Auth{User: cfg.SASLUser, Pass: cfg.SASLPass}.AsSha512Mechanism()))
	}
	return opts
}
