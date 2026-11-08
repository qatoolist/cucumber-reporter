package cucumber

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

func utf8Reader(r io.Reader) (*bufio.Reader, error) {
	b := bufio.NewReader(r)
	prefix, _ := b.Peek(3)
	if bytes.HasPrefix(prefix, []byte{0xff, 0xfe}) || bytes.HasPrefix(prefix, []byte{0xfe, 0xff}) {
		return nil, fmt.Errorf("%w: UTF-16 is unsupported; convert input to UTF-8", ErrInvalidJSON)
	}
	if bytes.HasPrefix(prefix, []byte{0xef, 0xbb, 0xbf}) {
		_, _ = b.Discard(3)
	}
	return b, nil
}

func wireTypeError(err error) error {
	var e *json.UnmarshalTypeError
	if errors.As(err, &e) {
		field := "field"
		// Only fixed schema fields are reported; unknown content is never echoed.
		for _, name := range []string{"duration", "line", "name", "status", "uri", "steps", "elements", "hidden"} {
			if strings.HasSuffix(e.Field, "."+name) || e.Field == name {
				field = name
				break
			}
		}
		detail := field + " has the wrong JSON type"
		if field == "duration" {
			detail = "duration must be integer nanoseconds within int64 range"
		}
		if field == "line" {
			detail = "line must be an integer"
		}
		return atOffset(fmt.Errorf("%w: %s", ErrInvalidJSON, detail), e.Offset)
	}
	return ErrInvalidJSON
}
