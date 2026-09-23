# gomailer

[![CI](https://github.com/eliasmeireles/gomailer/actions/workflows/ci.yml/badge.svg)](https://github.com/eliasmeireles/gomailer/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A lightweight Go worker that consumes email requests from RabbitMQ and delivers them through SMTP or a provider HTTP API (Resend, Zoho Mail, ZeptoMail), reporting the outcome to optional success/failure callbacks.

## Features

- **Queue-driven**: consumes JSON messages from a RabbitMQ queue with auto-reconnect and exponential backoff
- **Pluggable transports**: SMTP (implicit TLS) or HTTP API clients selected by configuration
- **API clients**: `resend`, `zoho` (Zoho Mail API, OAuth 2.0) and `zeptomail`, with a registry for adding new ones
- **Delivery callbacks**: optional per-message success and failure callbacks with the email `id`, `subject`, status and cause
- **Attachments and recipients**: base64-encoded attachments; `receiver`, `cc` and `bcc` as a comma-separated string or an array
- **Health probes**: `/healthz` (liveness) and `/readyz` (RabbitMQ readiness)
- **Container ready**: small multi-arch image configured only through environment variables

## How It Works

```
producer ──► RabbitMQ (mailer-service) ──► MailerConsumer ──► mailer.Service ──► Sender (smtp | resend | zoho | zeptomail)
                                                                   │
                                                                   ├── on success ──► DeliveryNotifier ──► callback.success.url
                                                                   └── on failure ──► DeliveryNotifier ──► callback.failure.url
```

| Outcome | Queue action |
|---|---|
| Email sent | ack (`callback.success` notified; if it fails, only logged — never resent) |
| Send failed, `callback.failure` answered 2xx | ack (failure handed to the callback) |
| Send failed, no `callback.failure` | nack + requeue |
| Send failed, `callback.failure` failed (non-2xx / unreachable) | nack + requeue |
| Invalid JSON message | nack + requeue |

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
| `message_invalid_body` | `body` is not valid base64 |
| `message_missing_sender` / `message_missing_receiver` / `message_missing_subject` / `message_missing_body` | Required field empty |
| `smtp_connection_failed` | TCP/TLS connection to the SMTP server failed |
| `smtp_authorization_denied` | SMTP credentials rejected |
| `smtp_sender_rejected` | `MAIL FROM` rejected (sender not allowed) |
| `smtp_receiver_rejected` | `RCPT TO` rejected for a receiver/cc/bcc address |
| `smtp_message_rejected` | Server rejected the message content |
| `api_connection_failed` | Provider API unreachable (DNS, network, timeout) |
| `api_authorization_denied` | Invalid/revoked API key or OAuth credentials |
| `api_sender_not_allowed` | Sender domain not verified or `from` not allowed |
| `api_invalid_receiver` | Invalid `to`/`cc`/`bcc` address |
| `api_invalid_attachment` | Attachment rejected by the provider |
| `api_invalid_request` | Other request validation errors |
| `api_quota_exceeded` | Daily/monthly quota or credits exhausted |
| `api_rate_limited` | Too many requests |
| `api_provider_unavailable` | Provider 5xx |
| `api_unexpected_response` | Any other provider response |
| `unknown_error` | Unclassified failure |

API codes are normalized across providers from their documented errors (Resend error names, ZeptoMail `TM_`/`SM_` codes, Zoho Mail responses), falling back to the HTTP status.

## Configuration

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
| `HEALTH_PORT` | `8080` | Port for `/healthz` and `/readyz` |

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
- Use `/healthz` as the liveness probe and `/readyz` (RabbitMQ connected) as the readiness/startup probe on `HEALTH_PORT`.
- Run a single transport per deployment; switch transports by changing `MAILER_TRANSPORT` / `MAILER_API_CLIENT`.

## Contributing

Issues and pull requests are welcome. Run `go test ./...` (and `cd .dev/console && go test ./...` when touching the dev console) and use the [local stack](.dev/README.md) to check a change end to end.

## License

[MIT](LICENSE)

## Security

- Credentials come only from environment variables
- TLS for SMTP and HTTPS for provider APIs
- Callback URLs come from the message: only trusted producers should publish to the queue
