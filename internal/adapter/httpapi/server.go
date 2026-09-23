package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/eliasmeireles/gomailer/internal/application/config"
)

const (
	readHeaderTimeout = 10 * time.Second
	shutdownTimeout   = 10 * time.Second
)

// Server is the HTTP source. It implements the runner Source contract (Name, Run, Ready).
type Server struct {
	addr    string
	handler http.Handler
	ready   atomic.Bool
}

// NewServer creates the HTTP source with POST /v1/emails protected by the configured keys;
// newID generates ids for requests without one.
func NewServer(cfg config.HTTPAPIConfig, service Deliverer, newID func() string) *Server {
	emails := &emailHandler{service: service, maxBodyBytes: cfg.MaxBodyBytes, newID: newID}

	mux := http.NewServeMux()
	mux.Handle("POST /v1/emails", requireBearer(cfg.Keys, emails))

	return &Server{addr: ":" + cfg.Port, handler: mux}
}

// Name identifies the source in logs.
func (s *Server) Name() string { return "http" }

// Ready reports whether the server is accepting connections.
func (s *Server) Ready() bool { return s.ready.Load() }

// Handler returns the routes, for tests.
func (s *Server) Handler() http.Handler { return s.handler }

// Run serves until ctx is cancelled (graceful shutdown) or the listener fails.
func (s *Server) Run(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("http source: listen on %s: %w", s.addr, err)
	}

	server := &http.Server{Handler: s.handler, ReadHeaderTimeout: readHeaderTimeout}
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(listener) }()

	s.ready.Store(true)
	log.Infof("HTTP source listening on %s (POST /v1/emails)", s.addr)
	defer s.ready.Store(false)

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("http source: %w", err)
	}
}
