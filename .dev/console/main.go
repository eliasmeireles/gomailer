// Command console is the gomailer dev console: a web form that publishes test emails to the
// mailer queue and panels that follow the stack (status, Mailpit inbox, failure callbacks).
package main

import (
	"log"
	"net/http"
	"time"

	"github.com/eliasmeireles/gomailer/dev/console/internal/config"
	"github.com/eliasmeireles/gomailer/dev/console/internal/message"
	"github.com/eliasmeireles/gomailer/dev/console/internal/monitor"
	"github.com/eliasmeireles/gomailer/dev/console/internal/queue"
	"github.com/eliasmeireles/gomailer/dev/console/internal/service"
	"github.com/eliasmeireles/gomailer/dev/console/internal/web"
)

func main() {
	cfg := config.Load()

	console := service.NewConsole(
		queue.NewPublisher(cfg.RabbitMQURL, cfg.Queue),
		monitor.NewMailpit(cfg.MailpitURL),
		monitor.NewCallbacks(cfg.CallbackServerURL),
		monitor.NewHealth(cfg.MailerHealthURL),
		message.NewID,
	)

	server, err := web.NewServer(console, web.Settings{
		MailerEnv:          cfg.MailerEnv,
		Queue:              cfg.Queue,
		MailpitPublicURL:   cfg.MailpitPublicURL,
		CallbackSuccessURL: cfg.CallbackSuccessURL,
		CallbackFailureURL: cfg.CallbackFailureURL,
	})
	if err != nil {
		log.Fatalf("console: %v", err)
	}

	httpServer := &http.Server{Addr: ":" + cfg.Port, Handler: server.Routes(), ReadHeaderTimeout: 5 * time.Second}
	log.Printf("console listening on :%s (queue %s, env %s)", cfg.Port, cfg.Queue, cfg.MailerEnv)
	log.Fatal(httpServer.ListenAndServe())
}
