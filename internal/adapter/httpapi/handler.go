// Package httpapi exposes the synchronous HTTP source: POST /v1/emails delivers the email in the
// request and answers with the delivery event.
package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/eliasmeireles/gomailer/internal/core/mailer"
	"github.com/eliasmeireles/gomailer/internal/core/model"
)

// Deliverer is what the handler needs from the service layer.
type Deliverer interface {
	Deliver(data model.SendEmailData) mailer.Outcome
}

// errorResponse is returned for requests rejected before delivery (auth, size, malformed JSON).
type errorResponse struct {
	Error string `json:"error"`
}

type emailHandler struct {
	service      Deliverer
	maxBodyBytes int64
	newID        func() string
}

// ServeHTTP decodes a SendEmailData, assigns an id when missing and delivers it synchronously.
// The response body is the model.DeliveryEvent; the status follows statusFor.
func (h *emailHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.maxBodyBytes)

	var data model.SendEmailData
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeDecodeError(w, err)
		return
	}
	if strings.TrimSpace(data.ID) == "" {
		data.ID = h.newID()
	}

	outcome := h.service.Deliver(data)
	writeJSON(w, statusFor(outcome), outcome.Event)
}

func writeDecodeError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		writeJSON(w, http.StatusRequestEntityTooLarge, errorResponse{Error: "request body too large"})
		return
	}
	writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body: " + err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
