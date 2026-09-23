package bean

import (
	"github.com/eliasmeireles/gomailer/internal/adapter/consumer"
	"github.com/eliasmeireles/gomailer/internal/infrastructure/messaging"
)

var (
	// RabbitMQConsumer is the RabbitMQ consumer instance
	RabbitMQConsumer *messaging.Consumer

	// MailerConsumer is the mailer consumer that processes email messages
	MailerConsumer *consumer.MailerConsumer
)

// InitMessaging initializes the RabbitMQ consumer and mailer consumer.
func InitMessaging() {
	config := messaging.NewRabbitMQConfig()
	RabbitMQConsumer = messaging.NewConsumer(config)
	MailerConsumer = consumer.NewMailerConsumer(MailerService)
}
