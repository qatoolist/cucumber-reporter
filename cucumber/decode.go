package cucumber

import (
	"encoding/base64"

	modelmeta "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"
)

// Sentinel errors. Test them with errors.Is. Messages never contain input content.
var (
	// ErrNoResults: no input artifacts were supplied. Valid empty arrays are accepted.
	ErrNoResults error
	// ErrInvalidJSON: the input is not one JSON array of features, has a value of the wrong type,
	// a duplicate object key, nesting deeper than 64 levels or data after the array.
	ErrInvalidJSON error
	// ErrIncomplete: an executed step has no result status. Empty feature arrays and
	// features with no scenarios are accepted; a scenario without steps is undefined.
	ErrIncomplete error
	// ErrInvalid: a status outside the six Godog statuses, or a negative duration.
	ErrInvalid error
	// ErrTooLarge: the input or one attachment exceeds its limit.
	ErrTooLarge error
	// ErrInvalidBase64: an attachment is not canonical standard base64 (Embedding.Decode only).
	ErrInvalidBase64 error
)

// Preserve the parser sentinel identities while keeping internal package names
// out of the public declarations and godoc.
func init() {
	ErrNoResults = modelmeta.ErrNoResults
	ErrInvalidJSON = modelmeta.ErrInvalidJSON
	ErrIncomplete = modelmeta.ErrIncomplete
	ErrInvalid = modelmeta.ErrInvalid
	ErrTooLarge = modelmeta.ErrTooLarge
	ErrInvalidBase64 = modelmeta.ErrInvalidBase64
}

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
