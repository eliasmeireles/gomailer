package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewID(t *testing.T) {
	t.Run("must return distinct version 4 uuids", func(t *testing.T) {
		first, second := NewID(), NewID()

		assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`, first)
		assert.NotEqual(t, first, second)
	})
}
