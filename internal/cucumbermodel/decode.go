package cucumbermodel

import (
	"encoding/base64"
	"errors"
)

// Sentinel errors. Test them with errors.Is. Messages never contain input content.
var (
	// ErrNoResults: no input artifacts were supplied. Valid empty arrays are accepted.
	ErrNoResults = errors.New("cucumber: input contains no features")
	// ErrInvalidJSON: the input is not one JSON array of features, has a value of the wrong type,
	// a duplicate object key, nesting deeper than 64 levels or data after the array.
	ErrInvalidJSON = errors.New("cucumber: input is not valid cucumber JSON")
	// ErrIncomplete: a feature has no scenarios or a step has no status. (A scenario without steps is
	// accepted: Godog writes it that way and counts it as undefined.)
	ErrIncomplete = errors.New("cucumber: input contains a feature without scenarios or a step without a status")
	// ErrInvalid: a status outside the six Godog statuses, or a negative duration.
	ErrInvalid = errors.New("cucumber: input contains an invalid value")
	// ErrTooLarge: the input or one attachment exceeds its limit.
	ErrTooLarge = errors.New("cucumber: input exceeds a size limit")
	// ErrInvalidBase64: an attachment is not canonical standard base64 (Embedding.Decode only).
	ErrInvalidBase64 = errors.New("cucumber: attachment is not canonical standard base64")
)

// Decode returns the attachment bytes. The data must be canonical standard base64 (RFC 4648 section 4
// with padding, no line breaks or other characters, zero padding bits): exactly what Godog writes and
// what the decoded bytes re-encode to. ErrTooLarge is returned, before decoding, when the bytes would
// exceed maxBytes.
func (e Embedding) Decode(maxBytes int64) ([]byte, error) {
	if e.Data == "" {
		return []byte{}, nil
	}
	if len(e.Data)%4 != 0 {
		return nil, ErrInvalidBase64
	}
	if int64(base64.StdEncoding.DecodedLen(len(e.Data))-padding(e.Data)) > maxBytes {
		return nil, ErrTooLarge
	}
	raw, err := base64.StdEncoding.Strict().DecodeString(e.Data)
	if err != nil || base64.StdEncoding.EncodeToString(raw) != e.Data {
		return nil, ErrInvalidBase64
	}
	return raw, nil
}

// padding counts trailing '=' characters, at most two.
func padding(s string) int {
	n := 0
	for i := len(s) - 1; i >= 0 && n < 2 && s[i] == '='; i-- {
		n++
	}
	return n
}
