package messaging

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
)

const (
	envRabbitMQUser  = "RABBITMQ_USER"
	envRabbitMQPass  = "RABBITMQ_PASS"
	envRabbitMQHost  = "RABBITMQ_HOST"
	envRabbitMQPort  = "RABBITMQ_PORT"
	envRabbitMQVHost = "RABBITMQ_VHOST"
	envRabbitMQQueue = "RABBITMQ_QUEUE"

	defaultRabbitMQPort  = "5672"
	defaultRabbitMQVHost = "/"
	defaultRabbitMQQueue = "mailer-service"
)

// RabbitMQConfig holds the RabbitMQ connection configuration.
type RabbitMQConfig struct {
	URL   string
	Queue string
}

// NewRabbitMQConfig creates a RabbitMQConfig from environment variables.
// Credentials (RABBITMQ_USER, RABBITMQ_PASS) are required.
func NewRabbitMQConfig() RabbitMQConfig {
	user := getRequiredEnv(envRabbitMQUser)
	pass := getRequiredEnv(envRabbitMQPass)
	host := getRequiredEnv(envRabbitMQHost)
	port := getEnvOrDefault(envRabbitMQPort, defaultRabbitMQPort)
	vhost := getEnvOrDefault(envRabbitMQVHost, defaultRabbitMQVHost)
	queue := getEnvOrDefault(envRabbitMQQueue, defaultRabbitMQQueue)

	if vhost != "" && vhost[0] != '/' {
		vhost = "/" + vhost
	}

	url := fmt.Sprintf("amqp://%s:%s@%s:%s%s", user, pass, host, port, vhost)

	return RabbitMQConfig{
		URL:   url,
		Queue: queue,
	}
}

func getRequiredEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("Required environment variable %s is not set", key)
	}
	return value
}

func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
