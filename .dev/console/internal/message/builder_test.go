package message

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixedID() string { return "fixed-id" }

func newTestForm() Form {
	return Form{
		From:    "no-reply@example.com",
		To:      "jane@example.com, john@example.com",
		Subject: "Subject",
		HTML:    "<p>Hello</p>",
		Format:  FormatArray,
	}
}

func TestBuild(t *testing.T) {
	t.Run("given a minimal form then build an email with generated id and base64 body", func(t *testing.T) {
		email, err := Build(newTestForm(), fixedID)

		require.NoError(t, err)
		assert.Equal(t, Email{
			ID:       "fixed-id",
			From:     "no-reply@example.com",
			Receiver: []string{"jane@example.com", "john@example.com"},
			Subject:  "Subject",
			Body:     base64.StdEncoding.EncodeToString([]byte("<p>Hello</p>")),
		}, email)
	})

	t.Run("given an id then keep it", func(t *testing.T) {
		form := newTestForm()
		form.ID = " order-1 "

		email, err := Build(form, fixedID)

		require.NoError(t, err)
		assert.Equal(t, "order-1", email.ID)
	})

	t.Run("given the string format then join recipients with comma", func(t *testing.T) {
		form := newTestForm()
		form.Format = FormatString
		form.Cc = "peter@example.com;anna@example.com"

		email, err := Build(form, fixedID)

		require.NoError(t, err)
		assert.Equal(t, "jane@example.com, john@example.com", email.Receiver)
		assert.Equal(t, "peter@example.com, anna@example.com", email.Cc)
		assert.Nil(t, email.Bcc)
	})

	t.Run("given files then encode them as base64 attachments", func(t *testing.T) {
		form := newTestForm()
		form.Files = []File{{Name: "a.txt", Type: "text/plain", Content: []byte("hi")}, {Name: "b.bin", Content: []byte{1}}}

		email, err := Build(form, fixedID)

		require.NoError(t, err)
		assert.Equal(t, []Attachment{
			{Name: "a.txt", Type: "text/plain", Data: "aGk=", Decoder: "base64"},
			{Name: "b.bin", Type: "application/octet-stream", Data: "AQ==", Decoder: "base64"},
		}, email.Attachments)
	})

	t.Run("given invalid body then send a non-base64 body even without html", func(t *testing.T) {
		form := newTestForm()
		form.HTML = ""
		form.InvalidBody = true

		email, err := Build(form, fixedID)

		require.NoError(t, err)
		assert.Equal(t, invalidBody, email.Body)
	})

	t.Run("given both callbacks with authorization then include them", func(t *testing.T) {
		form := newTestForm()
		form.SuccessCallbackEnabled = true
		form.SuccessCallbackURL = "http://callback:9099/success"
		form.FailureCallbackEnabled = true
		form.FailureCallbackURL = "http://callback:9099/failures"
		form.CallbackAuthorization = "Bearer token"

		email, err := Build(form, fixedID)

		require.NoError(t, err)
		headers := map[string]string{"Authorization": "Bearer token"}
		assert.Equal(t, &Callback{
			Success: &CallbackTarget{URL: "http://callback:9099/success", Headers: headers},
			Failure: &CallbackTarget{URL: "http://callback:9099/failures", Headers: headers},
		}, email.Callback)
	})

	t.Run("given only the failure callback without authorization then include only it", func(t *testing.T) {
		form := newTestForm()
		form.FailureCallbackEnabled = true
		form.FailureCallbackURL = "http://callback:9099/failures"

		email, err := Build(form, fixedID)

		require.NoError(t, err)
		assert.Equal(t, &Callback{Failure: &CallbackTarget{URL: "http://callback:9099/failures"}}, email.Callback)
	})

	t.Run("given disabled callbacks then omit them", func(t *testing.T) {
		form := newTestForm()
		form.SuccessCallbackURL = "http://callback:9099/success"
		form.FailureCallbackURL = "http://callback:9099/failures"

		email, err := Build(form, fixedID)

		require.NoError(t, err)
		assert.Nil(t, email.Callback)
	})

	t.Run("must omit empty optional fields in json", func(t *testing.T) {
		email, err := Build(newTestForm(), fixedID)
		require.NoError(t, err)

		raw, err := json.Marshal(email)

		require.NoError(t, err)
		assert.NotContains(t, string(raw), `"cc"`)
		assert.NotContains(t, string(raw), `"bcc"`)
		assert.NotContains(t, string(raw), `"callback"`)
		assert.NotContains(t, string(raw), `"attachments"`)
	})
}

func TestBuildValidation(t *testing.T) {
	cases := map[string]struct {
		mutate   func(*Form)
		expected string
	}{
		"given no from then reject":                      {func(f *Form) { f.From = " " }, "sender (from) is required"},
		"given no recipient then reject":                 {func(f *Form) { f.To = " , ;" }, "at least one recipient (to) is required"},
		"given no subject then reject":                   {func(f *Form) { f.Subject = "" }, "subject is required"},
		"given no html then reject":                      {func(f *Form) { f.HTML = "" }, "HTML body is required"},
		"given success callback without url then reject": {func(f *Form) { f.SuccessCallbackEnabled = true }, "success callback URL is required"},
		"given failure callback without url then reject": {func(f *Form) { f.FailureCallbackEnabled = true }, "failure callback URL is required"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			form := newTestForm()
			tc.mutate(&form)

			_, err := Build(form, fixedID)

			require.EqualError(t, err, tc.expected)
		})
	}
}

func TestSplitAddresses(t *testing.T) {
	t.Run("given commas, semicolons and new lines then split and trim", func(t *testing.T) {
		assert.Equal(t,
			[]string{"a@example.com", "b@example.com", "c@example.com"},
			SplitAddresses(" a@example.com ;b@example.com\n\n c@example.com, "),
		)
	})

	t.Run("given a blank list then return nil", func(t *testing.T) {
		assert.Nil(t, SplitAddresses(" \n "))
	})
}

func TestNewID(t *testing.T) {
	t.Run("must return distinct version 4 uuids", func(t *testing.T) {
		first, second := NewID(), NewID()

		assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`, first)
		assert.NotEqual(t, first, second)
	})
}
