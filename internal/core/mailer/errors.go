package mailer

import (
	"errors"
	"fmt"

	"github.com/eliasmeireles/gomailer/internal/core/model"
)

// DeliveryError attaches a model.ErrorCode to a delivery failure. It survives wrapping with %w,
// so the code can be recovered anywhere up the chain with CodeOf.
type DeliveryError struct {
	Code model.ErrorCode
	Err  error
}

func (e *DeliveryError) Error() string { return e.Err.Error() }

func (e *DeliveryError) Unwrap() error { return e.Err }

// NewError classifies err with code.
func NewError(code model.ErrorCode, err error) error {
	return &DeliveryError{Code: code, Err: err}
}

// Errorf formats an error (supporting %w) classified with code.
//
//	return mailer.Errorf(model.CodeSMTPAuthorizationDenied, "SMTP authentication failed: %w", err)
func Errorf(code model.ErrorCode, format string, args ...any) error {
	return NewError(code, fmt.Errorf(format, args...))
}

// CodeOf returns the code of the outermost DeliveryError in err's chain, or model.CodeUnknown.
func CodeOf(err error) model.ErrorCode {
	var deliveryErr *DeliveryError
	if errors.As(err, &deliveryErr) {
		return deliveryErr.Code
	}
	return model.CodeUnknown
}
