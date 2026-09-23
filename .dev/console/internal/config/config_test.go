package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoad(t *testing.T) {
	t.Run("given no env then use the compose defaults", func(t *testing.T) {
		for _, key := range []string{"CONSOLE_PORT", "RABBITMQ_URL", "RABBITMQ_QUEUE", "MAILPIT_URL", "MAILPIT_PUBLIC_URL",
			"CALLBACK_SERVER_URL", "CALLBACK_SUCCESS_URL", "CALLBACK_FAILURE_URL", "MAILER_HEALTH_URL", "MAILER_API_URL", "MAILER_API_KEY", "MAILER_ENV"} {
			t.Setenv(key, "")
		}

		assert.Equal(t, Config{
			Port:               "8080",
			RabbitMQURL:        "amqp://guest:guest@rabbitmq:5672/",
			Queue:              "mailer-service",
			MailpitURL:         "http://mailpit:8025",
			MailpitPublicURL:   "http://localhost:8025",
			CallbackServerURL:  "http://callback:9099",
			CallbackSuccessURL: "http://callback:9099/success",
			CallbackFailureURL: "http://callback:9099/failures",
			MailerHealthURL:    "http://mailer:8080/readyz",
			MailerAPIURL:       "http://mailer:8081",
			MailerAPIKey:       "dev-token",
			MailerEnv:          "smtp",
		}, Load())
	})

	t.Run("given overrides then use them and trim trailing slashes", func(t *testing.T) {
		t.Setenv("MAILER_ENV", "resend")
		t.Setenv("MAILPIT_URL", "http://localhost:8025/")

		cfg := Load()

		assert.Equal(t, "resend", cfg.MailerEnv)
		assert.Equal(t, "http://localhost:8025", cfg.MailpitURL)
	})
}
