package message

// RecipientsFormat selects how Receiver, Cc and Bcc are encoded in the message.
type RecipientsFormat string

const (
	// FormatArray sends recipients as a JSON array.
	FormatArray RecipientsFormat = "array"
	// FormatString sends recipients as a comma-separated string.
	FormatString RecipientsFormat = "string"
)

// File is an uploaded attachment with its raw content.
type File struct {
	Name    string
	Type    string
	Content []byte
}

// Form is the console input used to build an Email.
type Form struct {
	ID          string
	From        string
	To          string
	Cc          string
	Bcc         string
	Subject     string
	HTML        string
	Format      RecipientsFormat
	Files       []File
	InvalidBody bool

	SuccessCallbackEnabled bool
	SuccessCallbackURL     string
	FailureCallbackEnabled bool
	FailureCallbackURL     string
	// CallbackAuthorization is sent as the Authorization header of both callbacks.
	CallbackAuthorization string
}
