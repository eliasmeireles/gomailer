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
	RabbitMQAPIURL     string
	RabbitMQUser       string
	RabbitMQPass       string
	MockAPIURL         string
	MailpitURL         string
	MailpitPublicURL   string
	CallbackServerURL  string
	CallbackSuccessURL string
	CallbackFailureURL string
	MailerHealthURL    string
	MailerAPIURL       string
	MailerAPIKey       string
	MailerEnv          string
}

// Load reads the configuration, falling back to the compose defaults for unset variables.
func Load() Config {
	return Config{
		Port:               envOrDefault("CONSOLE_PORT", "8080"),
		RabbitMQURL:        envOrDefault("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/"),
		Queue:              envOrDefault("RABBITMQ_QUEUE", "mailer-service"),
		RabbitMQAPIURL:     trimSlash(envOrDefault("RABBITMQ_API_URL", "http://rabbitmq:15672")),
		RabbitMQUser:       envOrDefault("RABBITMQ_USER", "guest"),
		RabbitMQPass:       envOrDefault("RABBITMQ_PASS", "guest"),
		MockAPIURL:         trimSlash(envOrDefault("MOCK_API_URL", "http://mockapi:9100")),
		MailpitURL:         trimSlash(envOrDefault("MAILPIT_URL", "http://mailpit:8025")),
		MailpitPublicURL:   trimSlash(envOrDefault("MAILPIT_PUBLIC_URL", "http://localhost:8025")),
		CallbackServerURL:  trimSlash(envOrDefault("CALLBACK_SERVER_URL", "http://callback:9099")),
		CallbackSuccessURL: envOrDefault("CALLBACK_SUCCESS_URL", "http://callback:9099/success"),
		CallbackFailureURL: envOrDefault("CALLBACK_FAILURE_URL", "http://callback:9099/failures"),
		MailerHealthURL:    envOrDefault("MAILER_HEALTH_URL", "http://mailer:8080/readyz"),
		MailerAPIURL:       trimSlash(envOrDefault("MAILER_API_URL", "http://mailer:8081")),
		MailerAPIKey:       envOrDefault("MAILER_API_KEY", "dev-token"),
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
