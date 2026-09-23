SHELL := /bin/bash
.PHONY: build buildx run test update

IMAGE ?= ghcr.io/eliasmeireles/gomailer

update:
	@go mod tidy

test:
	@make update
	@go test -v -bench=. ./...

run:
	@set -a && { [ ! -f .env ] || source .env; } && set +a && go run ./cmd/server/

buildx:
	@docker buildx create --name buildxBuilder --use
	@docker buildx inspect buildxBuilder --bootstrap

build:
	@read -p "Enter the tag version: " TAG; \
	 docker buildx build --platform linux/amd64,linux/arm64 -t $(IMAGE):$$TAG --push .

# Local test stack (.dev). MAILER_ENV selects .dev/env/<env>.env (default: smtp -> Mailpit).
MAILER_ENV ?= smtp
MESSAGE ?= success
FROM ?= no-reply@exemplo.com.br
TO ?= maria@exemplo.com.br
CC ?=
BCC ?=
DEV_COMPOSE := MAILER_ENV=$(MAILER_ENV) docker compose -f .dev/docker-compose.yaml

.PHONY: dev-up dev-publish dev-logs dev-down

dev-up:
	@$(DEV_COMPOSE) up -d --build --force-recreate mailer console callback
	@echo "console: http://localhost:$${CONSOLE_PORT:-3000}  mailpit: http://localhost:$${MAILPIT_UI_PORT:-8025}"

dev-publish:
	@MAIL_FROM='$(FROM)' MAIL_TO='$(TO)' MAIL_CC='$(CC)' MAIL_BCC='$(BCC)' $(DEV_COMPOSE) run --rm publisher $(MESSAGE)

dev-logs:
	@$(DEV_COMPOSE) logs -f mailer callback

dev-down:
	@$(DEV_COMPOSE) down -v
