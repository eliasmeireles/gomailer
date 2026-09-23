package message

// RecipientsFormat selects how Receiver, Cc and Bcc are encoded in the message.
type RecipientsFormat string

const (
	// FormatArray sends recipients as a JSON array.
	FormatArray RecipientsFormat = "array"
	// FormatString sends recipients as a comma-separated string.
	FormatString RecipientsFormat = "string"
)

// Channel is how the console hands the email to the mailer.
type Channel string

const (
	// ChannelRabbitMQ publishes to the mailer queue (asynchronous).
	ChannelRabbitMQ Channel = "rabbitmq"
	// ChannelHTTP calls POST /v1/emails and waits for the delivery result.
	ChannelHTTP Channel = "http"
	// ChannelKafka produces to the mailer Kafka topic (asynchronous).
	ChannelKafka Channel = "kafka"
)

// File is an uploaded attachment with its raw content.
type File struct {
	Name    string
	Type    string
	Content []byte
}

// Form is the console input used to build an Email.
type Form struct {
	Channel     Channel
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
