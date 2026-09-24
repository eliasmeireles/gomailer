# gomailer Go client

`github.com/eliasmeireles/gomailer/client` is the Go library to send emails through a [gomailer](../README.md) instance. It holds the message contract and one sender per transport:

| Module / package | Go | Sender |
|---|---|---|
| `github.com/eliasmeireles/gomailer/client` | 1.22+ | Contract (`Email`, `Attachment`, `Callback`, `DeliveryEvent`, error codes), `Sender` interface and the synchronous `HTTPSender` |
| `github.com/eliasmeireles/gomailer/client/rabbitmq` (same module) | 1.22+ | Publishes to the gomailer queue with publisher confirms; reconnects lazily after a drop |
| `github.com/eliasmeireles/gomailer/client/kafka` (own module) | 1.26+ | Produces to the gomailer topic, keyed by the email ID (TLS and SASL PLAIN/SCRAM supported) |
| `github.com/eliasmeireles/gomailer/client/transport` (own module) | 1.26+ | Builds any of them from configuration (`transport.New` / `transport.FromEnv`) |

Kafka lives in its own module so RabbitMQ/HTTP users don't pull its dependencies (or its Go version requirement).

```bash
go get github.com/eliasmeireles/gomailer/client@latest            # contract, HTTP, RabbitMQ
go get github.com/eliasmeireles/gomailer/client/kafka@latest      # Kafka sender
go get github.com/eliasmeireles/gomailer/client/transport@latest  # transport chosen by configuration
```

## Usage

```go
sender, err := rabbitmq.New(rabbitmq.Config{
	URL:   "amqp://user:pass@rabbitmq:5672/notification",
	Queue: "mailer-service",
})
if err != nil {
	return err
}
defer sender.Close()

if err := sender.HealthCheck(ctx); err != nil { // connects and checks the queue, publishes nothing
	return err
}

err = sender.Send(ctx, client.Email{
	From:    "no-reply@exemplo.com.br",
	To:      []string{"maria@exemplo.com.br"},
	Subject: "Confirme sua conta",
	HTML:    renderedHTML, // plain HTML: the client base64-encodes it
})
```

- `Send` validates the email (`from`, at least one `to`, `subject` and `html`; errors wrap `client.ErrInvalidEmail`) and generates a UUID `ID` when empty.
- RabbitMQ and Kafka senders return once the broker acknowledged the message; gomailer delivers it asynchronously with retries. The HTTP sender returns once the email was delivered, or a `*client.DeliveryError` with the error code.
- All senders are safe for concurrent use.

### Choosing the transport by configuration

```go
cfg, err := transport.FromEnv() // GOMAILER_TRANSPORT=rabbitmq|kafka|http, GOMAILER_RABBITMQ_URL, ...
sender, err := transport.New(cfg)
```

See `transport.FromEnv` for every variable.

### Callbacks

```go
email.Callback = &client.Callback{
	Failure: &client.CallbackTarget{URL: "https://api.exemplo.com.br/mailer/failures", Headers: map[string]string{"Authorization": "Bearer " + token}},
}

http.HandleFunc("POST /mailer/failures", func(w http.ResponseWriter, r *http.Request) {
	event, err := client.ParseDeliveryEvent(r)
	// event.ID, event.ErrorCode (e.g. client.CodeAPIInvalidReceiver), event.Cause
})
```

## Migrating from a hand-written publisher

Projects that published `{"from","receiver","subject","body"}` (body base64) themselves can replace the publisher with a `client.Sender`:

| Before | After |
|---|---|
| `SendEmailData{From, Receiver, Subject, Body}` | `client.Email{From, To: []string{receiver}, Subject, HTML}` |
| `base64.StdEncoding.EncodeToString(html)` | not needed: pass the plain HTML |
| custom connect/reconnect/confirm code | `rabbitmq.New(rabbitmq.Config{URL, Queue})` |
| health check publishing `{}` into the queue | `sender.HealthCheck(ctx)` (passive queue check, nothing published) |

The wire format stays compatible with older gomailer/mailer-app consumers.

## Versioning

Each module is tagged with its path prefix: `client/vX.Y.Z`, `client/kafka/vX.Y.Z` and `client/transport/vX.Y.Z`.
