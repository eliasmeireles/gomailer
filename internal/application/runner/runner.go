// Package runner starts the enabled sources and the health server, and stops everything when
// the context is cancelled or any of them fails.
package runner

import (
	"context"
	"errors"
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"
)

// Source is an inbound channel of email requests (RabbitMQ, HTTP, ...).
type Source interface {
	// Name identifies the source in logs and errors.
	Name() string
	// Run blocks until ctx is cancelled or the source fails.
	Run(ctx context.Context) error
	// Ready reports whether the source can currently receive requests.
	Ready() bool
}

// HealthServer serves the liveness/readiness endpoints until ctx is cancelled.
type HealthServer interface {
	Start(ctx context.Context) error
}

// Readiness is ready when every source is ready.
type Readiness []Source

// Ready implements the health ReadyChecker.
func (r Readiness) Ready() bool {
	for _, source := range r {
		if !source.Ready() {
			return false
		}
	}
	return len(r) > 0
}

// Run starts health and every source concurrently. The first failure cancels the others; Run
// returns once all of them stopped, with the failures joined (nil on a clean shutdown).
func Run(ctx context.Context, health HealthServer, sources []Source) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
	)
	start := func(name string, run func(context.Context) error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				log.Errorf("%s stopped: %v", name, err)
				mu.Lock()
				errs = append(errs, fmt.Errorf("%s: %w", name, err))
				mu.Unlock()
				cancel()
			}
		}()
	}

	start("health", health.Start)
	for _, source := range sources {
		log.Infof("Starting source %s", source.Name())
		start(source.Name(), source.Run)
	}

	wg.Wait()
	return errors.Join(errs...)
}
