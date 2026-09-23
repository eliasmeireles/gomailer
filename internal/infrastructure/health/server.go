package health

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
)

const (
	envHealthPort     = "HEALTH_PORT"
	defaultHealthPort = "8080"

	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 5 * time.Second
)

// ReadyChecker reports whether the dependent component is ready.
type ReadyChecker interface {
	Ready() bool
}

// Server exposes /healthz (liveness) and /readyz (readiness) endpoints.
type Server struct {
	addr    string
	checker ReadyChecker
	server  *http.Server
}

// NewServer creates a health server bound to HEALTH_PORT (default 8080).
func NewServer(checker ReadyChecker) *Server {
	port := os.Getenv(envHealthPort)
	if port == "" {
		port = defaultHealthPort
	}
	return &Server{
		addr:    ":" + port,
		checker: checker,
	}
}

// Handler builds the http.Handler exposing the health endpoints.
// Exposed for tests; Start uses it internally.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.healthz)
	mux.HandleFunc("/readyz", s.readyz)
	return mux
}

// Start runs the HTTP server until the context is cancelled or the server fails.
// Returns nil on graceful shutdown.
func (s *Server) Start(ctx context.Context) error {
	s.server = &http.Server{
		Addr:              s.addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: readHeaderTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Infof("Health server listening on %s", s.addr)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return s.server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintln(w, "ok")
}

func (s *Server) readyz(w http.ResponseWriter, _ *http.Request) {
	if s.checker.Ready() {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintln(w, "ready")
		return
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = fmt.Fprintln(w, "not ready")
}
