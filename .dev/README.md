# Local Test Stack

Runs the whole flow in Docker, using the same image that is deployed (built from the repo `Dockerfile`):

| Service | Purpose | URL |
|---|---|---|
| `rabbitmq` | Queue `mailer-service` | UI http://localhost:15672 (`guest` / `guest`) |
| `mailer` | The app, configured by `env/<MAILER_ENV>.env` | Health http://localhost:8089/readyz |
| `mailpit` | Fake SMTP server with implicit TLS, catches every email | UI http://localhost:8025 |
| `console` | Web console: publishes emails and follows status, callbacks and inbox | http://localhost:3000 |
| `callback` | Receives success/failure callbacks (`/success`, `/failures`), prints and lists them (`GET /events`) | http://localhost:9099 |
| `certs` | One-shot: generates the test CA and Mailpit certificate | — |
| `publisher` | One-shot: publishes messages from `messages/` | — |

The mailer trusts the test CA through `SSL_CERT_DIR`, so the SMTP client verifies Mailpit's certificate exactly as it verifies a real server.

## Quick Start (no credentials needed)

```bash
make dev-up                 # MAILER_ENV=smtp -> Mailpit
open http://localhost:3000  # dev console
make dev-logs               # mailer + callback logs
make dev-down               # stop everything and drop volumes
```

## Dev Console

A Go web app (`console/`) that acts as an external producer: it builds messages in the mailer contract and publishes them straight to the queue over AMQP.

- Form with `id`, `from`, `to`/`cc`/`bcc` (sent as array or comma-separated string), subject, HTML body with live preview, attachments, success/failure callbacks and a "simulate failure" switch (non-base64 body).
- Header pills with the selected `MAILER_ENV`, mailer readiness, queue size and consumers.
- Panels for the received callbacks (sent/failed with cause) and the Mailpit inbox (to/cc/bcc, attachments), refreshed every 3s.

Its defaults follow `MAILER_ENV` (e.g. `from` is `onboarding@resend.dev` for Resend). Run its tests with `cd .dev/console && go test ./...`.

## Environments

`MAILER_ENV` selects `env/<name>.env`, which holds **only the mailer configuration** (the same variables used in the Kubernetes manifests):

| `MAILER_ENV` | Transport | File |
|---|---|---|
| `smtp` (default) | SMTP -> local Mailpit | `env/smtp.env` (committed) |
| `smtp-resend` | SMTP -> `smtp.resend.com:465` | copy `env/smtp-resend.env.example` |
| `smtp-zoho` | SMTP -> `smtp.zoho.com:465` | copy `env/smtp-zoho.env.example` |
| `resend` | Resend API | copy `env/resend.env.example` |
| `zoho` | Zoho Mail API (OAuth) | copy `env/zoho.env.example` |
| `zeptomail` | ZeptoMail API | copy `env/zeptomail.env.example` |

```bash
cp .dev/env/resend.env.example .dev/env/resend.env   # fill in the key; *.env files are git-ignored
MAILER_ENV=resend make dev-up
```

## Command-Line Publishing

The sender and recipient are part of each message, as a real producer sends them. Set them per publish:

```bash
make dev-publish FROM=onboarding@resend.dev TO=maria@exemplo.com.br
make dev-publish CC=joao@exemplo.com.br BCC=ana@exemplo.com.br
make dev-publish MESSAGE="success invalid-body"
```

| Variable | Default | Description |
|---|---|---|
| `MESSAGE` | `success` | One or more templates from `messages/` (space-separated) |
| `FROM` | `no-reply@exemplo.com.br` | Message `from` |
| `TO` | `maria@exemplo.com.br` | Message `receiver` (comma-separated for many) |
| `CC` / `BCC` | empty | Message `cc` / `bcc` (comma-separated, optional) |
| `MAILER_ENV` | `smtp` | Must match the one used in `dev-up` |

Available messages:

| Message | What it tests |
|---|---|
| `success` | HTML body + attachment + optional cc/bcc + both callbacks. Expect delivery and the **success** callback |
| `array-recipients` | Same as `success`, with `receiver`/`cc`/`bcc` sent as JSON arrays |
| `invalid-body` | Body is not base64. Expect the **failure** callback with the cause, message acked |
| `invalid-body-no-callback` | Same failure without callback. The message is requeued (and retried continuously) |

Provider rules for `FROM`: Resend without a verified domain only accepts `onboarding@resend.dev` sending to the account owner's email; Zoho requires an address/alias of the account; ZeptoMail requires a verified domain.

## Testing the Callback Failure Path

```bash
CALLBACK_STATUS=500 MAILER_ENV=smtp docker compose -f .dev/docker-compose.yaml up -d callback
make dev-publish MESSAGE=invalid-body   # callback answers 500 -> message goes back to the queue
```

## Useful Overrides

Host ports can be changed with `RABBITMQ_PORT`, `RABBITMQ_UI_PORT`, `MAILPIT_UI_PORT`, `CALLBACK_PORT` and `MAILER_HEALTH_PORT`.
