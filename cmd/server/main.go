package main

import (
	"context"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"

	"github.com/eliasmeireles/gomailer/internal/application/bean"
	"github.com/eliasmeireles/gomailer/internal/application/runner"
	"github.com/eliasmeireles/gomailer/internal/infrastructure/health"
)

func main() {
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})

	bean.InitMailer()
	sources := bean.InitSources()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := runner.Run(ctx, health.NewServer(runner.Readiness(sources)), sources); err != nil {
		log.Fatalf("Mailer stopped: %v", err)
	}
	log.Info("Shutdown complete")
}
