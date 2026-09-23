package decoder

import "encoding/base64"

type Base64 struct {
}

func NewBase64() *Base64 {
	return &Base64{}
}

func (b *Base64) Decode(data string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(data)
}

// init registers the base64 decoder implementation in the decoder registry.
// This allows other packages to obtain the decoder via GetDecoder(TypeBase64).
func init() {
	// Ignore return values; registration happens once during init.
	_, _ = Register(TypeBase64, NewBase64())
}
