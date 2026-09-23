package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eliasmeireles/gomailer/dev/console/internal/monitor"
)

func TestConsoleChaos(t *testing.T) {
	t.Run("given both sides available then return their state", func(t *testing.T) {
		f := newFakes()
		f.chaos.triggers = monitor.ChaosTriggers{Sender: monitor.ChaosTrigger{ErrorCode: 451, Probability: 100}}
		f.mock.state = monitor.MockState{Accepted: 2}

		view := f.console(true).Chaos(context.Background())

		assert.Equal(t, ChaosView{SMTP: f.chaos.triggers, Mock: f.mock.state}, view)
	})

	t.Run("given unavailable sides then report each error", func(t *testing.T) {
		f := newFakes()
		f.chaos.err = errors.New("mailpit down")
		f.mock.err = errors.New("mock down")

		view := f.console(true).Chaos(context.Background())

		assert.Equal(t, "mailpit down", view.SMTPError)
		assert.Equal(t, "mock down", view.MockError)
	})

	t.Run("must set smtp triggers and configure the mock", func(t *testing.T) {
		f := newFakes()
		triggers := monitor.ChaosTriggers{Recipient: monitor.ChaosTrigger{ErrorCode: 550, Probability: 100}}
		behavior := monitor.MockBehavior{FailNext: 2, Status: 429}
		console := f.console(true)

		require.NoError(t, console.SetSMTPChaos(context.Background(), triggers))
		require.NoError(t, console.ConfigureMockAPI(context.Background(), behavior))

		assert.Equal(t, triggers, f.chaos.triggers)
		assert.Equal(t, behavior, f.mock.state.MockBehavior)
	})

	t.Run("given reset then turn every trigger off keeping codes and reset the mock", func(t *testing.T) {
		f := newFakes()
		f.chaos.triggers = monitor.ChaosTriggers{
			Sender:         monitor.ChaosTrigger{ErrorCode: 451, Probability: 100},
			Authentication: monitor.ChaosTrigger{ErrorCode: 535, Probability: 100},
		}

		require.NoError(t, f.console(true).ResetChaos(context.Background()))

		assert.Equal(t, monitor.ChaosTriggers{
			Sender:         monitor.ChaosTrigger{ErrorCode: 451},
			Authentication: monitor.ChaosTrigger{ErrorCode: 535},
		}, f.chaos.triggers)
		assert.True(t, f.mock.reset)
	})

	t.Run("given reset failures then join both errors", func(t *testing.T) {
		f := newFakes()
		f.chaos.err = errors.New("mailpit down")
		f.mock.resetErr = errors.New("mock down")

		err := f.console(true).ResetChaos(context.Background())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "mailpit down")
		assert.Contains(t, err.Error(), "mock down")
	})
}
