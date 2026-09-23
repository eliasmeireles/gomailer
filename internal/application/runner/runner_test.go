package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSource struct {
	name  string
	ready bool
	err   error
	ran   chan struct{}
}

func newFakeSource(name string, ready bool, err error) *fakeSource {
	return &fakeSource{name: name, ready: ready, err: err, ran: make(chan struct{})}
}

func (f *fakeSource) Name() string { return f.name }
func (f *fakeSource) Ready() bool  { return f.ready }
func (f *fakeSource) Run(ctx context.Context) error {
	close(f.ran)
	if f.err != nil {
		return f.err
	}
	<-ctx.Done()
	return ctx.Err()
}

type fakeHealth struct{}

func (fakeHealth) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

func TestReadiness(t *testing.T) {
	t.Run("given all sources ready then ready", func(t *testing.T) {
		assert.True(t, Readiness{newFakeSource("a", true, nil), newFakeSource("b", true, nil)}.Ready())
	})

	t.Run("given one source not ready then not ready", func(t *testing.T) {
		assert.False(t, Readiness{newFakeSource("a", true, nil), newFakeSource("b", false, nil)}.Ready())
	})

	t.Run("given no sources then not ready", func(t *testing.T) {
		assert.False(t, Readiness{}.Ready())
	})
}

func TestRun(t *testing.T) {
	t.Run("given a cancelled context then stop every source cleanly", func(t *testing.T) {
		a, b := newFakeSource("a", true, nil), newFakeSource("b", true, nil)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)

		go func() { done <- Run(ctx, fakeHealth{}, []Source{a, b}) }()
		<-a.ran
		<-b.ran
		cancel()

		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(2 * time.Second):
			require.Fail(t, "runner did not stop")
		}
	})

	t.Run("given a failing source then stop the others and return its error", func(t *testing.T) {
		healthy := newFakeSource("healthy", true, nil)
		failing := newFakeSource("failing", false, errors.New("port in use"))

		err := Run(context.Background(), fakeHealth{}, []Source{healthy, failing})

		require.EqualError(t, err, "failing: port in use")
	})
}
