package decoder

import (
	"errors"
	"strings"
)

// Type defines model for Type.
type Type string

// Defines values for Type.
const (
	TypeBase64 Type = "base64"
)

type Decoder interface {
	Decode(data string) ([]byte, error)
}

var decoders = map[Type]Decoder{}

// Register registers a new decoder.
func Register(decoderType Type, decoder Decoder) (bool, error) {
	if _, ok := decoders[decoderType]; ok {
		return false, errors.New("decoder already registered")
	}

	decoders[decoderType] = decoder
	return true, nil
}

// GetDecoder returns a decoder for the given type.
func GetDecoder(decoderType Type) (Decoder, error) {
	if decoder, ok := decoders[decoderType]; ok {
		return decoder, nil
	}

	availableDecoders := make([]string, 0, len(decoders))
	for decoderType := range decoders {
		availableDecoders = append(availableDecoders, string(decoderType))
	}

	return nil, errors.New("decoder not found, available decoders: " + strings.Join(availableDecoders, ", "))
}
