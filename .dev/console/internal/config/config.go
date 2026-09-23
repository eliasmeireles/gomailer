// Package config loads the dev console settings from the environment.
package config

import (
	"os"
	"strings"
)

// Config holds the endpoints the console talks to. Defaults match the .dev compose network.
type Config struct {
	Port               string
	RabbitMQURL        string
	Queue              string
	MailpitURL         string
	MailpitPublicURL   string
	CallbackServerURL  string
	CallbackSuccessURL string
	CallbackFailureURL string
	MailerHealthURL    string
	MailerEnv          string
}

// Load reads the configuration, falling back to the compose defaults for unset variables.
func Load() Config {
	return Config{
		Port:               envOrDefault("CONSOLE_PORT", "8080"),
		RabbitMQURL:        envOrDefault("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/"),
		Queue:              envOrDefault("RABBITMQ_QUEUE", "mailer-service"),
		MailpitURL:         trimSlash(envOrDefault("MAILPIT_URL", "http://mailpit:8025")),
		MailpitPublicURL:   trimSlash(envOrDefault("MAILPIT_PUBLIC_URL", "http://localhost:8025")),
		CallbackServerURL:  trimSlash(envOrDefault("CALLBACK_SERVER_URL", "http://callback:9099")),
		CallbackSuccessURL: envOrDefault("CALLBACK_SUCCESS_URL", "http://callback:9099/success"),
		CallbackFailureURL: envOrDefault("CALLBACK_FAILURE_URL", "http://callback:9099/failures"),
		MailerHealthURL:    envOrDefault("MAILER_HEALTH_URL", "http://mailer:8080/readyz"),
		MailerEnv:          envOrDefault("MAILER_ENV", "smtp"),
	}
}

func envOrDefault(key, defaultValue string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return defaultValue
}

func trimSlash(url string) string {
	return strings.TrimRight(url, "/")
}
