package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRecipients(t *testing.T) {
	t.Run("given a single address then return it", func(t *testing.T) {
		assert.Equal(t, Recipients{"a@example.com"}, ParseRecipients("a@example.com"))
	})

	t.Run("given spaced and empty entries then return a clean list", func(t *testing.T) {
		assert.Equal(t,
			Recipients{"a@example.com", "b@example.com", "c@example.com"},
			ParseRecipients(" a@example.com ,, b@example.com , ,c@example.com "),
		)
	})

	t.Run("given a blank string then return nil", func(t *testing.T) {
		assert.Nil(t, ParseRecipients("  "))
	})
}

func TestRecipientsUnmarshalJSON(t *testing.T) {
	cases := map[string]struct {
		input    string
		expected Recipients
	}{
		"given a single address string then return one recipient":     {`"a@example.com"`, Recipients{"a@example.com"}},
		"given a comma-separated string then split it":                {`"a@example.com, b@example.com"`, Recipients{"a@example.com", "b@example.com"}},
		"given an array then keep each address":                       {`["a@example.com", " b@example.com "]`, Recipients{"a@example.com", "b@example.com"}},
		"given an array with comma-separated items then flatten them": {`["a@example.com,b@example.com", "c@example.com"]`, Recipients{"a@example.com", "b@example.com", "c@example.com"}},
		"given an array with blank items then drop them":              {`["", "a@example.com", "  "]`, Recipients{"a@example.com"}},
		"given an empty array then return nil":                        {`[]`, nil},
		"given null then return nil":                                  {`null`, nil},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var recipients Recipients

			require.NoError(t, json.Unmarshal([]byte(tc.input), &recipients))
			assert.Equal(t, tc.expected, recipients)
		})
	}

	t.Run("given a non-string value then return error", func(t *testing.T) {
		var recipients Recipients

		err := json.Unmarshal([]byte(`42`), &recipients)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "recipients must be a string or an array of strings")
	})

	t.Run("given an array with non-string items then return error", func(t *testing.T) {
		var recipients Recipients

		require.Error(t, json.Unmarshal([]byte(`["a@example.com", 1]`), &recipients))
	})
}

func TestRecipientsJSONRoundTrip(t *testing.T) {
	t.Run("given a message with string recipients then marshal them as arrays", func(t *testing.T) {
		var data SendEmailData
		require.NoError(t, json.Unmarshal([]byte(`{"receiver":"a@example.com, b@example.com","cc":["c@example.com"]}`), &data))

		out, err := json.Marshal(data)

		require.NoError(t, err)
		assert.Contains(t, string(out), `"receiver":["a@example.com","b@example.com"]`)
		assert.Contains(t, string(out), `"cc":["c@example.com"]`)
		assert.NotContains(t, string(out), `"bcc"`)
	})
}

func TestRecipientsString(t *testing.T) {
	t.Run("must join addresses with comma and space", func(t *testing.T) {
		assert.Equal(t, "a@example.com, b@example.com", Recipients{"a@example.com", "b@example.com"}.String())
	})
}
