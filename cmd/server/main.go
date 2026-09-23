package main

import (
	"context"
	"errors"
	"os/signal"
	"sync"
	"syscall"

	log "github.com/sirupsen/logrus"

	"github.com/eliasmeireles/gomailer/internal/application/bean"
	"github.com/eliasmeireles/gomailer/internal/infrastructure/health"
)

func main() {
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})

	bean.InitMailer()
	bean.InitMessaging()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	healthSrv := health.NewServer(bean.RabbitMQConsumer)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := healthSrv.Start(ctx); err != nil {
			log.Errorf("Health server stopped: %v", err)
		}
	}()

	if err := bean.RabbitMQConsumer.Connect(ctx); err != nil {
		if !errors.Is(err, context.Canceled) {
			log.Errorf("RabbitMQ connect aborted: %v", err)
		}
	} else {
		defer bean.RabbitMQConsumer.Close()

		log.Info("Mailer consumer started, waiting for messages...")

		if err := bean.RabbitMQConsumer.Consume(ctx, bean.MailerConsumer.Handle); err != nil &&
			!errors.Is(err, context.Canceled) {
			log.Errorf("RabbitMQ consumer error: %v", err)
		}
	}

	stop()
	wg.Wait()
	log.Info("Shutdown complete")
}
