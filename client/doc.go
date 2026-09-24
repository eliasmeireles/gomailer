// Package client is the Go client of gomailer: the message contract and the senders that hand
// emails to a gomailer instance through RabbitMQ, Kafka or its HTTP API.
//
// Build an Email with plain HTML and send it with any Sender:
//
//	sender, err := transport.New(transport.Config{
//		Transport: transport.RabbitMQ,
//		RabbitMQ:  rabbitmq.Config{URL: "amqp://user:pass@rabbitmq:5672/notification", Queue: "mailer-service"},
//	})
//	...
//	err = sender.Send(ctx, client.Email{
//		From:    "no-reply@example.com",
//		To:      []string{"jane@example.com"},
//		Subject: "Welcome",
//		HTML:    "<h1>Hello, Jane</h1>",
//	})
//
// The client encodes the wire format (base64 body and attachments, recipient arrays) and
// generates an ID when none is given.
package client
