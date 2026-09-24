# gomailer

[![CI](https://github.com/eliasmeireles/gomailer/actions/workflows/ci.yml/badge.svg)](https://github.com/eliasmeireles/gomailer/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A lightweight Go email service that receives requests from RabbitMQ, Kafka or an HTTP API and delivers them through SMTP or a provider HTTP API (Resend, Zoho Mail, ZeptoMail), reporting the outcome to optional success/failure callbacks.

## Features

- **Multiple sources**: RabbitMQ queue and Kafka topic, plus a synchronous HTTP API (`POST /v1/emails`) enabled by default (`HTTP_API_DISABLED=true` turns it off)
- **Retries and dead-letter queue**: temporary failures are retried with exponential backoff; final unhandled failures go to a DLQ
- **Pluggable transports**: SMTP (implicit TLS) or HTTP API clients selected by configuration
- **API clients**: `resend`, `zoho` (Zoho Mail API, OAuth 2.0) and `zeptomail`, with a registry for adding new ones
- **Delivery callbacks**: optional per-message success and failure callbacks with the email `id`, `subject`, status and cause
- **Attachments and recipients**: base64-encoded attachments; `receiver`, `cc` and `bcc` as a comma-separated string or an array
- **Health probes**: `/healthz` (liveness) and `/readyz` (RabbitMQ readiness)
- **Container ready**: small multi-arch image configured only through environment variables

## How It Works

```
producer ──► RabbitMQ (mailer-service) ──► MailerConsumer ──┐
producer ──► Kafka (mailer-service) ─────► MailerConsumer ──┤
client   ──► POST /v1/emails (HTTP API) ───────────────────┴──► mailer.Service ──► Sender (smtp | resend | zoho | zeptomail)
                                                                   │
                                                                   ├── on success ──► DeliveryNotifier ──► callback.success.url
                                                                   └── on failure ──► DeliveryNotifier ──► callback.failure.url
```

Every source uses the same message contract, callbacks and error codes.

### Retries and Dead-Letter Queue

Queued requests that fail with a **temporary** error code (`api_connection_failed`, `api_rate_limited`, `api_provider_unavailable`, `smtp_connection_failed`, `smtp_temporary_failure`) are retried up to `MAILER_MAX_ATTEMPTS` times, waiting `MAILER_RETRY_BASE_DELAY` doubled per attempt (default 30s, 1m, 2m, 4m). Callbacks are only called with the final outcome.

| Outcome | RabbitMQ | Kafka |
|---|---|---|
| Email sent | ack | commit |
| Temporary failure with attempts left | republished to `<queue>.retry.<delay>`, then ack | produced to `<topic>.retry` with `x-attempt` and `x-not-before`, then commit |
| Final failure, `callback.failure` answered 2xx | ack | commit |
| Final failure, no `callback.failure` or it failed | republished to `<queue>.dlq`, then ack | produced to `<topic>.dlq`, then commit |
| Invalid JSON message | `<queue>.dlq` (`message_invalid_json`) | `<topic>.dlq` (`message_invalid_json`) |

`callback.success` is notified on success (a failing success callback is only logged, never resent). Dead letters carry the `x-error-code`, `x-error-cause`, `x-attempt` and `x-failed-at` headers.

The retry queues need no plugin: each one has a message TTL equal to its delay and dead-letters expired messages back to the main queue; the delay is part of the name (`mailer-service.retry.30s`), so changing the policy creates new queues instead of conflicting with existing ones. Retries and dead letters are republished with publisher confirms; if the broker does not confirm, the original message is requeued.

On Kafka, the group `<group>` consumes `<topic>` and `<group>-retry` consumes `<topic>.retry`, processing each retry once its `x-not-before` time has passed. Offsets are committed only after a record is resolved (sent, handed to the failure callback, or produced to the retry/dead-letter topic), and rebalances are blocked while a batch is in flight, so the same record is not delivered by two members at once. Delivery is at-least-once: a crash between sending and committing may send an email again. The key of the original record is kept, so retries stay on the same partition.

The HTTP API never retries: it answers the first attempt.

## Queue Message

Messages are read from `RABBITMQ_QUEUE` (default `mailer-service`). The `body` is **base64-encoded HTML**.

```json
{
  "id": "order-123-confirmation",
  "from": "no-reply@example.com",
  "receiver": ["jane@example.com", "john@example.com"],
  "cc": "peter@example.com",
  "bcc": ["anna@example.com"],
  "subject": "Email Subject",
  "body": "PGgxPkhlbGxvPC9oMT4=",
  "attachments": [
    { "name": "document.pdf", "data": "base64EncodedData...", "type": "application/pdf", "decoder": "base64" }
  ],
  "callback": {
    "success": { "url": "https://example.com/mailer/sent", "headers": { "Authorization": "Bearer <token>" } },
    "failure": { "url": "https://example.com/mailer/failures", "headers": { "Authorization": "Bearer <token>" } }
  }
}
```

| Field | Required | Description |
|---|---|---|
| `id` | no | Producer-defined identifier, echoed back in the callbacks |
| `from` | yes | Sender address. Must belong to a verified domain / the provider account |
| `receiver` | yes | Recipients: a comma-separated string (`"a@x.com, b@x.com"`) or an array (`["a@x.com", "b@x.com"]`) |
| `cc` | no | Carbon-copy recipients, string or array |
| `bcc` | no | Blind-copy recipients, string or array (never exposed in the message headers) |
| `subject` | yes | Email subject |
| `body` | yes | Base64-encoded HTML |
| `attachments[]` | no | `name`, `type` (MIME), `data` and optional `decoder` (default `base64`). Undecodable attachments are skipped |
| `callback.success` | no | `url` and optional `headers` notified after the email is sent |
| `callback.failure` | no | `url` and optional `headers` notified when delivery fails |

### Delivery Callbacks

Both callbacks are optional and independent. The service sends `POST <url>` with `Content-Type: application/json` plus the target `headers`. The payload has the same shape for both, so a single URL can handle them:

```json
{ "id": "order-123-confirmation", "subject": "Email Subject", "status": "sent", "occurredAt": "2026-09-23T12:00:00Z" }
```

```json
{
  "id": "order-123-confirmation",
  "subject": "Email Subject",
  "status": "failed",
  "errorCode": "api_sender_not_allowed",
  "cause": "failed to send email to jane@example.com: resend API returned status 403: validation_error: The example.com domain is not verified.",
  "occurredAt": "2026-09-23T12:00:00Z"
}
```

The payload only identifies the email (`id` and `subject`) to keep callbacks small; the producer correlates it with the original message through the `id`.

### Error Codes

Failure events carry a stable `errorCode` for programmatic handling; `cause` keeps the human-readable detail (including the provider's own error).

| Code | When |
|---|---|
| `message_invalid_json` | Queued message is not valid JSON (dead-lettered) |
| `message_invalid_body` | `body` is not valid base64 |
| `message_missing_sender` / `message_missing_receiver` / `message_missing_subject` / `message_missing_body` | Required field empty |
| `smtp_connection_failed` | TCP/TLS connection to the SMTP server failed — retried |
| `smtp_authorization_denied` | SMTP credentials rejected |
| `smtp_sender_rejected` | `MAIL FROM` rejected (sender not allowed) |
| `smtp_receiver_rejected` | `RCPT TO` rejected for a receiver/cc/bcc address |
| `smtp_message_rejected` | Server rejected the message content |
| `smtp_temporary_failure` | Any 4xx SMTP reply (e.g. 421, 450, 451) — retried |
| `api_connection_failed` | Provider API unreachable (DNS, network, timeout) — retried |
| `api_authorization_denied` | Invalid/revoked API key or OAuth credentials |
| `api_sender_not_allowed` | Sender domain not verified or `from` not allowed |
| `api_invalid_receiver` | Invalid `to`/`cc`/`bcc` address |
| `api_invalid_attachment` | Attachment rejected by the provider |
| `api_invalid_request` | Other request validation errors |
| `api_quota_exceeded` | Daily/monthly quota or credits exhausted |
| `api_rate_limited` | Too many requests — retried |
| `api_provider_unavailable` | Provider 5xx — retried |
| `api_unexpected_response` | Any other provider response |
| `unknown_error` | Unclassified failure |

API codes are normalized across providers from their documented errors (Resend error names, ZeptoMail `TM_`/`SM_` codes, Zoho Mail responses), falling back to the HTTP status.

## HTTP API

The HTTP source runs by default next to the queue sources in `MAILER_SOURCES`; `MAILER_SOURCES=http` runs it alone and `HTTP_API_DISABLED=true` turns it off. The request body is the same [queue message](#queue-message); the email is delivered synchronously and the response body is the delivery event.

```bash
curl -X POST http://localhost:8081/v1/emails \
  -H "Authorization: Bearer $HTTP_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"id":"order-123","from":"no-reply@example.com","receiver":["jane@example.com"],"subject":"Hello","body":"PGgxPkhlbGxvPC9oMT4="}'
```

```json
{ "id": "order-123", "subject": "Hello", "status": "sent", "occurredAt": "2026-09-23T12:00:00Z" }
```

| Status | When |
|---|---|
| `200` | Sent |
| `400` | `message_*` error codes, or malformed JSON |
| `401` | Missing or invalid Bearer token |
| `413` | Body larger than `HTTP_API_MAX_BODY_BYTES` |
| `422` | Rejected by the provider (`api_invalid_receiver`, `api_sender_not_allowed`, `smtp_receiver_rejected`...) |
| `429` | `api_rate_limited` |
| `503` | Provider unreachable or unavailable (`*_connection_failed`, `api_provider_unavailable`) |
| `502` | Credentials rejected or unexpected provider response |

Requests without `id` get a generated UUID, returned in the response. Callbacks in the body are honored as with the queue. The endpoint always requires a Bearer token: while the HTTP source is enabled, the service refuses to start without `HTTP_API_KEYS`. Expose it only to trusted clients (it sends email from your domain).

## Go Client

Go applications can use the client library instead of writing their own publisher: [`client/`](client/README.md) (`go get github.com/eliasmeireles/gomailer/client`). It encodes the message contract and sends through RabbitMQ, Kafka or the HTTP API:

```go
sender, _ := rabbitmq.New(rabbitmq.Config{URL: "amqp://user:pass@rabbitmq:5672/notification", Queue: "mailer-service"})
err := sender.Send(ctx, client.Email{From: "no-reply@example.com", To: []string{"jane@example.com"}, Subject: "Hello", HTML: "<p>Hello</p>"})
```

## Configuration

### Sources

| Variable | Default | Description |
|---|---|---|
| `MAILER_SOURCES` | `rabbitmq` | Comma-separated sources to enable: `rabbitmq`, `kafka`, `http`. The HTTP source is added automatically; list `http` alone to run only it |
| `HTTP_API_DISABLED` | `false` | `true` turns the HTTP source off (no `POST /v1/emails` endpoint, no `HTTP_API_KEYS` needed) |
| `HTTP_API_PORT` | `8081` | HTTP source port |
| `HTTP_API_KEYS` | — | Required unless `HTTP_API_DISABLED=true`: comma-separated accepted Bearer tokens |
| `HTTP_API_MAX_BODY_BYTES` | `26214400` | Max request body (25 MiB) |

### Kafka (`MAILER_SOURCES` includes `kafka`)

| Variable | Default | Description |
|---|---|---|
| `KAFKA_BROKERS` | — | Required: comma-separated bootstrap brokers |
| `KAFKA_TOPIC` | `mailer-service` | Main topic (retry: `<topic>.retry`, dead-letter: `<topic>.dlq`) |
| `KAFKA_GROUP_ID` | `gomailer` | Consumer group (the retry topic uses `<group>-retry`) |
| `KAFKA_CREATE_TOPICS` | `false` | Create missing topics on startup |
| `KAFKA_TOPIC_PARTITIONS` / `KAFKA_TOPIC_REPLICATION` | `3` / `1` | Used only when creating topics |
| `KAFKA_TLS` | `false` | Connect with TLS |
| `KAFKA_SASL_MECHANISM` | — | `plain`, `scram-sha-256` or `scram-sha-512` |
| `KAFKA_SASL_USER` / `KAFKA_SASL_PASS` | — | SASL credentials (secret) |

Messages are the same JSON as the [queue message](#queue-message); use the email `id` as record key to keep related emails ordered.

### Retries

| Variable | Default | Description |
|---|---|---|
| `MAILER_MAX_ATTEMPTS` | `5` | Attempts per queued request (`1` disables retries) |
| `MAILER_RETRY_BASE_DELAY` | `30s` | Wait after the first failed attempt, doubled per attempt |
| `MAILER_RETRY_MAX_DELAY` | `10m` | Upper bound of the wait |

### Transport

| Variable | Default | Description |
|---|---|---|
| `MAILER_TRANSPORT` | `smtp` | `smtp` or `api` |
| `MAILER_API_CLIENT` | — | Required when `MAILER_TRANSPORT=api`: `resend`, `zoho` or `zeptomail` |
| `HTTP_CLIENT_TIMEOUT` | `15s` | Timeout for provider API and callback calls (Go duration) |

### SMTP (`MAILER_TRANSPORT=smtp`)

| Variable | Description |
|---|---|
| `SMTP_SERVER` | SMTP host (e.g. `smtp.zoho.com`, `smtp.resend.com`) |
| `SMTP_SERVER_PORT` | Implicit-TLS port, usually `465` |
| `SMTP_SERVER_USER` | SMTP user. For Resend: `resend` |
| `SMTP_SERVER_PASS` | SMTP password. For Resend: the API key |

### Resend (`MAILER_API_CLIENT=resend`)

| Variable | Default | Description |
|---|---|---|
| `RESEND_API_KEY` | — | API key with sending access |
| `RESEND_API_URL` | `https://api.resend.com` | API base URL |

### Zoho Mail (`MAILER_API_CLIENT=zoho`)

Uses the Zoho Mail API with an OAuth 2.0 refresh token; access tokens are refreshed and cached automatically. Attachments are uploaded before the message is sent.

| Variable | Default | Description |
|---|---|---|
| `ZOHO_MAIL_ACCOUNT_ID` | — | Mail account id, from `GET /api/accounts` |
| `ZOHO_CLIENT_ID` | — | OAuth client id |
| `ZOHO_CLIENT_SECRET` | — | OAuth client secret |
| `ZOHO_REFRESH_TOKEN` | — | OAuth refresh token |
| `ZOHO_MAIL_API_URL` | `https://mail.zoho.com` | Data-center Mail API host (e.g. `https://mail.zoho.eu`) |
| `ZOHO_ACCOUNTS_URL` | `https://accounts.zoho.com` | Data-center OAuth host (e.g. `https://accounts.zoho.eu`) |

Getting the credentials:

1. At [api-console.zoho.com](https://api-console.zoho.com), create a **Self Client**.
2. Generate a grant code with scopes `ZohoMail.messages.CREATE,ZohoMail.accounts.READ`.
3. Exchange it for a refresh token: `POST https://accounts.zoho.com/oauth/v2/token?grant_type=authorization_code&client_id=...&client_secret=...&code=...`.
4. Get the account id with `GET https://mail.zoho.com/api/accounts` (`Authorization: Zoho-oauthtoken <access_token>`).

### ZeptoMail (`MAILER_API_CLIENT=zeptomail`)

| Variable | Default | Description |
|---|---|---|
| `ZEPTOMAIL_API_KEY` | — | Send Mail token, with or without the `Zoho-enczapikey ` prefix |
| `ZEPTOMAIL_API_URL` | `https://api.zeptomail.com` | Region host (e.g. `https://api.zeptomail.eu`) |

### RabbitMQ and Health

| Variable | Default | Description |
|---|---|---|
| `RABBITMQ_HOST` | — | Broker host |
| `RABBITMQ_PORT` | `5672` | Broker port |
| `RABBITMQ_USER` / `RABBITMQ_PASS` | — | Credentials |
| `RABBITMQ_VHOST` | `/` | Virtual host |
| `RABBITMQ_QUEUE` | `mailer-service` | Queue consumed (declared durable) |
| `HEALTH_PORT` | `8080` | Port for `/healthz` and `/readyz` (ready when every enabled source is ready) |

## Adding a New API Client

1. Implement `mailer.Sender` in `internal/infrastructure/mailer` (reuse `validateEmail`, `allRecipients`, `decodeAttachments`, `newJSONRequest`, `doRequest` and `describeAPIError` with a classifier that maps the provider errors to error codes).
2. Add its configuration loader in `internal/application/config`.
3. Register a factory under a new name in `apiClients` (`internal/infrastructure/mailer/api_clients.go`).
4. Add an example env file under `.dev/env/` and document its variables here.

## Running Locally

The full stack (RabbitMQ, the mailer image, a local SMTP server, a callback receiver and a web console) runs with Docker Compose from [`.dev/`](.dev/README.md):

```bash
make dev-up                              # SMTP via local Mailpit, no credentials needed
open http://localhost:3000               # dev console: compose and publish emails, follow callbacks and inbox
MAILER_ENV=resend make dev-up            # or any provider: smtp-resend, smtp-zoho, resend, zoho, zeptomail
make dev-down
```

Unit tests: `make test`.

## Container Image

Public multi-arch images (`linux/amd64`, `linux/arm64`) are published to [`ghcr.io/eliasmeireles/gomailer`](https://github.com/eliasmeireles/gomailer/pkgs/container/gomailer) when a `vX.Y.Z` tag is pushed. No login is required.

| Tag | Points to |
|---|---|
| `X.Y.Z` (e.g. `1.0.0`) | That exact release (recommended for deployments) |
| `X.Y` (e.g. `1.0`) | Latest patch of that minor version |
| `latest` | Latest release |

```bash
docker pull ghcr.io/eliasmeireles/gomailer:1.0.0
```

The container exposes `8080` (health: `/healthz`, `/readyz`) and `8081` (`POST /v1/emails`, unless `HTTP_API_DISABLED=true`). All configuration comes from the [environment variables](#configuration).

### HTTP source + Resend API (no broker needed)

```bash
docker run -d --name gomailer \
  -e MAILER_SOURCES=http -e HTTP_API_KEYS=change-me \
  -e MAILER_TRANSPORT=api -e MAILER_API_CLIENT=resend -e RESEND_API_KEY=re_xxxxxxxxx \
  -p 8080:8080 -p 8081:8081 \
  ghcr.io/eliasmeireles/gomailer:1.0.0

curl -s localhost:8080/readyz
curl -X POST localhost:8081/v1/emails \
  -H "Authorization: Bearer change-me" -H "Content-Type: application/json" \
  -d '{"from":"no-reply@example.com","receiver":["jane@example.com"],"subject":"Hello","body":"PGgxPkhlbGxvPC9oMT4="}'
```

### RabbitMQ source + SMTP

Keep credentials out of your shell history with an env file. This example runs a queue worker only; drop `HTTP_API_DISABLED`, set `HTTP_API_KEYS` and publish `8081` to also accept HTTP requests:

```bash
cat > gomailer.env <<'ENV'
MAILER_SOURCES=rabbitmq
HTTP_API_DISABLED=true
MAILER_TRANSPORT=smtp
SMTP_SERVER=smtp.example.com
SMTP_SERVER_PORT=465
SMTP_SERVER_USER=no-reply@example.com
SMTP_SERVER_PASS=change-me
RABBITMQ_HOST=rabbitmq.example.com
RABBITMQ_USER=gomailer
RABBITMQ_PASS=change-me
RABBITMQ_QUEUE=mailer-service
ENV

docker run -d --name gomailer --env-file gomailer.env -p 8080:8080 ghcr.io/eliasmeireles/gomailer:1.0.0
docker logs -f gomailer
```

The RabbitMQ user needs configure on `<queue>`, `<queue>.retry.*` and `<queue>.dlq`; write on `amq.default`, `<queue>.retry.*` and `<queue>.dlq`; and read on `<queue>` and `<queue>.retry.*` (declaring a dead-lettering queue requires read).

### Docker Compose

```yaml
services:
  gomailer:
    image: ghcr.io/eliasmeireles/gomailer:1.0.0
    restart: unless-stopped
    env_file: gomailer.env
    ports:
      - "8080:8080"
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:8080/readyz"]
      interval: 15s
      timeout: 3s
      retries: 3
```

To build and push your own multi-arch image: `make build IMAGE=<registry>/<name>`.

## Deployment

gomailer is configured only through environment variables, so it runs anywhere containers run. In Kubernetes:

- Store credentials (`SMTP_SERVER_PASS`, `RESEND_API_KEY`, `ZOHO_*`, `ZEPTOMAIL_API_KEY`, `RABBITMQ_PASS`, `KAFKA_SASL_PASS`, `HTTP_API_KEYS`) in a Secret or an external secret manager, and the rest in plain env vars.
- Use `/healthz` as the liveness probe and `/readyz` (every enabled source ready) as the readiness/startup probe on `HEALTH_PORT`.
- When the HTTP source is enabled, keep its Service internal or behind an authenticated gateway, and store `HTTP_API_KEYS` as a secret.
- Run a single transport per deployment; switch transports by changing `MAILER_TRANSPORT` / `MAILER_API_CLIENT`.

## Contributing

Issues and pull requests are welcome. Run `go test ./...` (and `cd .dev/console && go test ./...` when touching the dev console) and use the [local stack](.dev/README.md) to check a change end to end.

## License

[MIT](LICENSE)

## Security

- Credentials come only from environment variables
- TLS for SMTP and HTTPS for provider APIs
- Callback URLs come from the message: only trusted producers should publish to the queue
