# Local Test Stack

Runs the whole flow in Docker, using the same image that is deployed (built from the repo `Dockerfile`):

| Service | Purpose | URL |
|---|---|---|
| `rabbitmq` | Queue `mailer-service` (+ retry queues and DLQ) | UI http://localhost:15672 (`guest` / `guest`) |
| `kafka` | Topics `mailer-service`, `.retry`, `.dlq` (KRaft, single node) | `localhost:9094` |
| `mailer` | The app, configured by `env/<MAILER_ENV>.env`, with the `rabbitmq`, `kafka` and `http` sources | Health http://localhost:8089/readyz · API http://localhost:8090/v1/emails (token `dev-token`) |
| `mailpit` | Fake SMTP server with implicit TLS, catches every email; chaos enabled to inject SMTP errors | UI http://localhost:8025 |
| `console` | Web console: publishes emails and follows status, callbacks and inbox | http://localhost:3000 |
| `mockapi` | Fake Resend-compatible API that fails on demand (`MAILER_ENV=resend-mock`) | http://localhost:9110/state |
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

- **Send via**: RabbitMQ (publishes to the queue), Kafka (produces to the topic) or HTTP (calls `POST /v1/emails` and shows the synchronous response: status, `errorCode`, `cause`).
- Form with `id`, `from`, `to`/`cc`/`bcc` (sent as array or comma-separated string), subject, HTML body with live preview, attachments, success/failure callbacks and a "simulate failure" switch (non-base64 body).
- Header pills with the selected `MAILER_ENV`, mailer readiness and, for RabbitMQ and Kafka, pending messages, consumers, messages waiting to be retried and in the DLQ.
- **Simulate failures**: Mailpit chaos per SMTP step (4xx temporary or 5xx permanent) and the mock API failing the next N requests with a Resend error (429, 503, 422, 403, 401).
- Panels for the dead letters of RabbitMQ and Kafka (source, error code, attempts, cause; purge), the received callbacks (sent/failed with errorCode and cause) and the Mailpit inbox (to/cc/bcc, attachments), refreshed every 3s.

Its defaults follow `MAILER_ENV` (e.g. `from` is `onboarding@resend.dev` for Resend). Run its tests with `cd .dev/console && go test ./...`.

## HTTP API

The mailer runs with `MAILER_SOURCES=rabbitmq,kafka`; the HTTP source is on by default. Call the API directly:

```bash
curl -X POST http://localhost:8090/v1/emails -H "Authorization: Bearer dev-token" -H "Content-Type: application/json" \
  -d '{"from":"no-reply@example.com","receiver":"jane@example.com","subject":"Hi","body":"PGgxPkhpPC9oMT4="}'
```

Override the sources or the token with `MAILER_SOURCES=http make dev-up` (HTTP only) and `HTTP_API_KEY=<token>`.

## Test Scenarios

The mailer runs with short retries in the stack (`MAILER_MAX_ATTEMPTS=3`, waits of 5s and 10s; override with the same variables). Use **Simulate failures** in the console, then send an email through any channel (RabbitMQ, Kafka or HTTP):

| Scenario | Setup | Expected |
|---|---|---|
| Temporary failure that recovers | `smtp`: recipient `451`, send, set it back to off within 5s · or `resend-mock`: `503`, fail next 2 | Retried, then delivered; success callback |
| Temporary failure that persists | `smtp`: recipient `451` · or `resend-mock`: `429`, fail next 5 | 3 attempts; failure callback with `smtp_temporary_failure` / `api_rate_limited`, or DLQ without callback |
| Permanent failure | `smtp`: recipient `550` · or `resend-mock`: `422` | No retry; failure callback with `smtp_receiver_rejected` / `api_invalid_receiver`, or DLQ |
| Invalid message | **Simulate failure (non-base64 body)** | `message_invalid_body`, no retry |
| Synchronous API | Send via HTTP with any failure | Immediate response with the mapped status (e.g. 503), no retry |
| Invalid JSON on Kafka | `echo '{nope' \| docker compose -f .dev/docker-compose.yaml exec -T kafka /opt/kafka/bin/kafka-console-producer.sh --bootstrap-server localhost:9092 --topic mailer-service` | Kafka DLQ with `message_invalid_json` |

## Environments

`MAILER_ENV` selects `env/<name>.env`, which holds **only the mailer configuration** (the same variables used in the Kubernetes manifests):

| `MAILER_ENV` | Transport | File |
|---|---|---|
| `smtp` (default) | SMTP -> local Mailpit | `env/smtp.env` (committed) |
| `resend-mock` | Resend API client -> local mock | `env/resend-mock.env` (committed) |
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
make dev-publish FROM=onboarding@resend.dev TO=jane@example.com
make dev-publish CC=john@example.com BCC=anna@example.com
make dev-publish MESSAGE="success invalid-body"
```

| Variable | Default | Description |
|---|---|---|
| `MESSAGE` | `success` | One or more templates from `messages/` (space-separated) |
| `FROM` | `no-reply@example.com` | Message `from` |
| `TO` | `jane@example.com` | Message `receiver` (comma-separated for many) |
| `CC` / `BCC` | empty | Message `cc` / `bcc` (comma-separated, optional) |
| `MAILER_ENV` | `smtp` | Must match the one used in `dev-up` |

Available messages:

| Message | What it tests |
|---|---|
| `success` | HTML body + attachment + optional cc/bcc + both callbacks. Expect delivery and the **success** callback |
| `array-recipients` | Same as `success`, with `receiver`/`cc`/`bcc` sent as JSON arrays |
| `invalid-body` | Body is not base64. Expect the **failure** callback with the cause, message acked |
| `invalid-body-no-callback` | Same failure without callback: the message goes to the DLQ |

Provider rules for `FROM`: Resend without a verified domain only accepts `onboarding@resend.dev` sending to the account owner's email; Zoho requires an address/alias of the account; ZeptoMail requires a verified domain.

## Testing the Callback Failure Path

```bash
CALLBACK_STATUS=500 MAILER_ENV=smtp docker compose -f .dev/docker-compose.yaml up -d callback
make dev-publish MESSAGE=invalid-body   # failure callback answers 500 -> message goes to the DLQ
```

## Useful Overrides

Host ports can be changed with `RABBITMQ_PORT`, `RABBITMQ_UI_PORT`, `MAILPIT_UI_PORT`, `CALLBACK_PORT`, `MOCKAPI_PORT`, `KAFKA_PORT`, `MAILER_HEALTH_PORT`, `MAILER_API_PORT` and `CONSOLE_PORT`.
