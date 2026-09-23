# gomailer

[![CI](https://github.com/eliasmeireles/gomailer/actions/workflows/ci.yml/badge.svg)](https://github.com/eliasmeireles/gomailer/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A lightweight Go email service that receives requests from RabbitMQ or an HTTP API and delivers them through SMTP or a provider HTTP API (Resend, Zoho Mail, ZeptoMail), reporting the outcome to optional success/failure callbacks.

## Features

- **Multiple sources**: RabbitMQ queue (auto-reconnect with backoff) and a synchronous HTTP API (`POST /v1/emails`), enabled together or alone
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
client   ──► POST /v1/emails (HTTP API) ───────────────────┴──► mailer.Service ──► Sender (smtp | resend | zoho | zeptomail)
                                                                   │
                                                                   ├── on success ──► DeliveryNotifier ──► callback.success.url
                                                                   └── on failure ──► DeliveryNotifier ──► callback.failure.url
```

Every source uses the same message contract, callbacks and error codes.

### Retries and Dead-Letter Queue

Queued requests that fail with a **temporary** error code (`api_connection_failed`, `api_rate_limited`, `api_provider_unavailable`, `smtp_connection_failed`, `smtp_temporary_failure`) are retried up to `MAILER_MAX_ATTEMPTS` times, waiting `MAILER_RETRY_BASE_DELAY` doubled per attempt (default 30s, 1m, 2m, 4m). Callbacks are only called with the final outcome.

| Outcome | RabbitMQ action |
|---|---|
| Email sent | ack; `callback.success` notified (a failing success callback is only logged, never resent) |
| Temporary failure with attempts left | republished to `<queue>.retry.<delay>`, then ack |
| Final failure, `callback.failure` answered 2xx | ack (failure handed to the callback) |
| Final failure, no `callback.failure` or it failed | republished to `<queue>.dlq` with `x-error-code`, `x-error-cause`, `x-attempt`, `x-failed-at` headers, then ack |
| Invalid JSON message | `<queue>.dlq` (`message_invalid_json`) |

The retry queues need no plugin: each one has a message TTL equal to its delay and dead-letters expired messages back to the main queue; the delay is part of the name (`mailer-service.retry.30s`), so changing the policy creates new queues instead of conflicting with existing ones. Retries and dead letters are republished with publisher confirms; if the broker does not confirm, the original message is requeued. The HTTP API never retries: it answers the first attempt.

## Queue Message

Messages are read from `RABBITMQ_QUEUE` (default `mailer-service`). The `body` is **base64-encoded HTML**.

```json
{
  "id": "pedido-123-confirmacao",
  "from": "no-reply@exemplo.com.br",
  "receiver": ["maria@exemplo.com.br", "joao@exemplo.com.br"],
  "cc": "pedro@exemplo.com.br",
  "bcc": ["ana@exemplo.com.br"],
  "subject": "Email Subject",
  "body": "PGgxPk9sw6E8L2gxPg==",
  "attachments": [
    { "name": "document.pdf", "data": "base64EncodedData...", "type": "application/pdf", "decoder": "base64" }
  ],
  "callback": {
    "success": { "url": "https://exemplo.com.br/mailer/sent", "headers": { "Authorization": "Bearer <token>" } },
    "failure": { "url": "https://exemplo.com.br/mailer/failures", "headers": { "Authorization": "Bearer <token>" } }
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
{ "id": "pedido-123-confirmacao", "subject": "Email Subject", "status": "sent", "occurredAt": "2026-09-23T12:00:00Z" }
```

```json
{
  "id": "pedido-123-confirmacao",
  "subject": "Email Subject",
  "status": "failed",
  "errorCode": "api_sender_not_allowed",
  "cause": "failed to send email to maria@exemplo.com.br: resend API returned status 403: validation_error: The exemplo.com.br domain is not verified.",
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

Enable it with `MAILER_SOURCES=http` (or `rabbitmq,http`). The request body is the same [queue message](#queue-message); the email is delivered synchronously and the response body is the delivery event.

```bash
curl -X POST http://localhost:8081/v1/emails \
  -H "Authorization: Bearer $HTTP_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"id":"pedido-123","from":"no-reply@exemplo.com.br","receiver":["maria@exemplo.com.br"],"subject":"Olá","body":"PGgxPk9sw6E8L2gxPg=="}'
```

```json
{ "id": "pedido-123", "subject": "Olá", "status": "sent", "occurredAt": "2026-09-23T12:00:00Z" }
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

Requests without `id` get a generated UUID, returned in the response. Callbacks in the body are honored as with the queue. The endpoint always requires a Bearer token: the service refuses to start the HTTP source without `HTTP_API_KEYS`. Expose it only to trusted clients (it sends email from your domain).

## Configuration

### Sources

| Variable | Default | Description |
|---|---|---|
| `MAILER_SOURCES` | `rabbitmq` | Comma-separated sources to enable: `rabbitmq`, `http` |
| `HTTP_API_PORT` | `8081` | HTTP source port |
| `HTTP_API_KEYS` | — | Required for `http`: comma-separated accepted Bearer tokens |
| `HTTP_API_MAX_BODY_BYTES` | `26214400` | Max request body (25 MiB) |

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

Released images are published to `ghcr.io/eliasmeireles/gomailer` (tags `X.Y.Z`, `X.Y` and `latest`) by pushing a `vX.Y.Z` tag.

```bash
docker run --rm \
  -e MAILER_TRANSPORT=api -e MAILER_API_CLIENT=resend -e RESEND_API_KEY=re_xxxxxxxxx \
  -e RABBITMQ_HOST=rabbitmq -e RABBITMQ_USER=guest -e RABBITMQ_PASS=guest \
  -p 8080:8080 ghcr.io/eliasmeireles/gomailer:latest
```

To build and push your own multi-arch image: `make build IMAGE=<registry>/<name>`.

## Deployment

gomailer is configured only through environment variables, so it runs anywhere containers run. In Kubernetes:

- Store credentials (`SMTP_SERVER_PASS`, `RESEND_API_KEY`, `ZOHO_*`, `ZEPTOMAIL_API_KEY`, `RABBITMQ_PASS`) in a Secret or an external secret manager, and the rest in plain env vars.
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
