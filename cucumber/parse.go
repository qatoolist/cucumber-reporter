package cucumber

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Sentinel errors. Test them with errors.Is. Messages never contain input content.
var (
	// ErrNoResults: the input is an empty array, or no features were supplied.
	ErrNoResults = errors.New("cucumber: input contains no features")
	// ErrInvalidJSON: the input is not one JSON array of features, has a value of the wrong type,
	// a duplicate object key, nesting deeper than 64 levels or data after the array.
	ErrInvalidJSON = errors.New("cucumber: input is not valid cucumber JSON")
	// ErrIncomplete: a feature has no scenarios, a scenario has no steps or a step has no status.
	ErrIncomplete = errors.New("cucumber: input contains a feature without scenarios, a scenario without steps or a step without a status")
	// ErrInvalid: a status outside the six Godog statuses, or a negative duration.
	ErrInvalid = errors.New("cucumber: input contains an invalid value")
	// ErrTooLarge: the input or one attachment exceeds its limit.
	ErrTooLarge = errors.New("cucumber: input exceeds a size limit")
	// ErrInvalidBase64: an attachment is not canonical standard base64 (Embedding.Decode only).
	ErrInvalidBase64 = errors.New("cucumber: attachment is not canonical standard base64")
)

// ParseError locates a failure. Path is built from array indices and fixed field names only, such as
// "feature[3].elements[1].steps[7]"; Offset is a byte offset into the input, or -1. Neither carries
// input content.
type ParseError struct {
	Path   string
	Offset int64
	Err    error
}

func (e *ParseError) Error() string {
	switch {
	case e.Path != "":
		return e.Err.Error() + " at " + e.Path
	case e.Offset >= 0:
		return e.Err.Error() + " at byte " + strconv.FormatInt(e.Offset, 10)
	}
	return e.Err.Error()
}

func (e *ParseError) Unwrap() error { return e.Err }

func atPath(err error, format string, args ...any) error {
	return &ParseError{Path: fmt.Sprintf(format, args...), Offset: -1, Err: err}
}

func atOffset(err error, offset int64) error {
	return &ParseError{Offset: offset, Err: err}
}

// maxDepth bounds JSON nesting. Cucumber JSON needs about ten levels.
const maxDepth = 64

// Wire structures. Pointers distinguish a missing or null value from an empty one.
type (
	wireFeature struct {
		URI         string          `json:"uri"`
		ID          string          `json:"id"`
		Keyword     string          `json:"keyword"`
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Line        int             `json:"line"`
		Tags        []wireTag       `json:"tags"`
		Elements    *[]wireScenario `json:"elements"`
	}
	wireScenario struct {
		ID          string      `json:"id"`
		Keyword     string      `json:"keyword"`
		Name        string      `json:"name"`
		Description string      `json:"description"`
		Line        int         `json:"line"`
		Type        string      `json:"type"`
		Tags        []wireTag   `json:"tags"`
		Steps       *[]wireStep `json:"steps"`
	}
	wireTag struct {
		Name string `json:"name"`
		Line int    `json:"line"`
	}
	wireStep struct {
		Keyword    string          `json:"keyword"`
		Name       string          `json:"name"`
		Line       int             `json:"line"`
		DocString  *wireDocString  `json:"doc_string"`
		Rows       []wireRow       `json:"rows"`
		Match      wireMatch       `json:"match"`
		Result     *wireResult     `json:"result"`
		Embeddings []wireEmbedding `json:"embeddings"`
	}
	wireDocString struct {
		Value       string `json:"value"`
		ContentType string `json:"content_type"`
		Line        int    `json:"line"`
	}
	wireRow struct {
		Cells []string `json:"cells"`
	}
	wireMatch struct {
		Location string `json:"location"`
	}
	wireResult struct {
		Status   *string `json:"status"`
		Error    string  `json:"error_message"`
		Duration *int64  `json:"duration"`
	}
	wireEmbedding struct {
		Name     string `json:"name"`
		MimeType string `json:"mime_type"`
		Data     string `json:"data"`
	}
)

// Parse reads legacy Cucumber JSON as written by Godog's cucumber formatter. It is strict: the input
// must be one non-empty array of complete features (every feature has scenarios, every scenario has
// steps and every step has one of the six statuses), with no duplicate object keys and nothing after
// the array. Unknown fields, such as Godog's feature comments or other runners' hooks, are ignored.
// Attachment data is kept as given and checked when it is decoded.
func Parse(r io.Reader, lim Limits) ([]Feature, error) {
	lim = lim.WithDefaults()
	data, err := io.ReadAll(io.LimitReader(r, lim.Input+1))
	if err != nil {
		return nil, fmt.Errorf("cucumber: read input: %w", err)
	}
	if int64(len(data)) > lim.Input {
		return nil, ErrTooLarge
	}
	if err := checkSyntax(data); err != nil {
		return nil, err
	}
	var wire *[]wireFeature
	if err := json.Unmarshal(data, &wire); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			return nil, atOffset(ErrInvalidJSON, typeErr.Offset)
		}
		return nil, ErrInvalidJSON
	}
	if wire == nil {
		return nil, ErrInvalidJSON
	}
	features, err := convert(*wire, lim)
	if err != nil {
		return nil, err
	}
	if err := Validate(features); err != nil {
		return nil, err
	}
	return features, nil
}

// checkSyntax walks the token stream once. It requires a top-level array and rejects duplicate keys
// (compared the way encoding/json matches field names, ignoring case), excessive nesting and trailing data.
func checkSyntax(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	first, err := dec.Token()
	if err != nil {
		return atOffset(ErrInvalidJSON, dec.InputOffset())
	}
	if d, ok := first.(json.Delim); !ok || d != '[' {
		return atOffset(ErrInvalidJSON, 0)
	}
	if err := walkContainer(dec, '[', 1); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return atOffset(ErrInvalidJSON, dec.InputOffset())
	}
	return nil
}

// walkContainer consumes the rest of an array or object whose opening delimiter has been read.
func walkContainer(dec *json.Decoder, open json.Delim, depth int) error {
	if depth > maxDepth {
		return atOffset(ErrInvalidJSON, dec.InputOffset())
	}
	var seen map[string]struct{}
	if open == '{' {
		seen = map[string]struct{}{}
	}
	for dec.More() {
		if open == '{' {
			tok, err := dec.Token()
			if err != nil {
				return atOffset(ErrInvalidJSON, dec.InputOffset())
			}
			key, ok := tok.(string)
			if !ok {
				return atOffset(ErrInvalidJSON, dec.InputOffset())
			}
			folded := foldKey(key)
			if _, dup := seen[folded]; dup {
				return atOffset(ErrInvalidJSON, dec.InputOffset())
			}
			seen[folded] = struct{}{}
		}
		tok, err := dec.Token()
		if err != nil {
			return atOffset(ErrInvalidJSON, dec.InputOffset())
		}
		if d, ok := tok.(json.Delim); ok {
			if err := walkContainer(dec, d, depth+1); err != nil {
				return err
			}
		}
	}
	if _, err := dec.Token(); err != nil { // closing delimiter
		return atOffset(ErrInvalidJSON, dec.InputOffset())
	}
	return nil
}

// foldKey maps every rune to the smallest rune of its simple case-folding orbit, so keys that
// encoding/json would treat as the same field ("name", "Name", "NAME") compare equal.
func foldKey(key string) string {
	return strings.Map(func(r rune) rune {
		smallest := r
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			if f < smallest {
				smallest = f
			}
		}
		return smallest
	}, key)
}

func convert(wire []wireFeature, lim Limits) ([]Feature, error) {
	features := make([]Feature, len(wire))
	for i, wf := range wire {
		f := Feature{URI: wf.URI, ID: wf.ID, Keyword: wf.Keyword, Name: wf.Name, Description: wf.Description, Line: wf.Line, Tags: tags(wf.Tags)}
		if wf.Elements == nil || len(*wf.Elements) == 0 {
			return nil, atPath(ErrIncomplete, "feature[%d]", i)
		}
		f.Elements = make([]Scenario, len(*wf.Elements))
		for j, ws := range *wf.Elements {
			sc := Scenario{ID: ws.ID, Keyword: ws.Keyword, Name: ws.Name, Description: ws.Description, Type: ws.Type, Line: ws.Line, Tags: tags(ws.Tags)}
			if ws.Steps == nil || len(*ws.Steps) == 0 {
				return nil, atPath(ErrIncomplete, "feature[%d].elements[%d]", i, j)
			}
			sc.Steps = make([]Step, len(*ws.Steps))
			for k, wst := range *ws.Steps {
				st, err := convertStep(wst, lim)
				if err != nil {
					var pe *ParseError
					if errors.As(err, &pe) {
						pe.Path = fmt.Sprintf("feature[%d].elements[%d].steps[%d]%s", i, j, k, pe.Path)
					}
					return nil, err
				}
				sc.Steps[k] = st
			}
			f.Elements[j] = sc
		}
		features[i] = f
	}
	return features, nil
}

// convertStep returns errors whose Path is relative to the step ("" for the step itself).
func convertStep(wst wireStep, lim Limits) (Step, error) {
	rel := func(err error, path string) error { return &ParseError{Path: path, Offset: -1, Err: err} }
	if wst.Result == nil || wst.Result.Status == nil {
		return Step{}, rel(ErrIncomplete, "")
	}
	st := Step{Keyword: wst.Keyword, Name: wst.Name, Line: wst.Line, Match: Match{Location: wst.Match.Location}}
	st.Result = Result{Status: Status(*wst.Result.Status), Error: wst.Result.Error}
	if !st.Result.Status.Valid() {
		return Step{}, rel(ErrInvalid, ".result.status")
	}
	if wst.Result.Duration != nil {
		if *wst.Result.Duration < 0 {
			return Step{}, rel(ErrInvalid, ".result.duration")
		}
		d := time.Duration(*wst.Result.Duration)
		st.Result.Duration = &d
	}
	if wst.DocString != nil {
		st.DocString = &DocString{Value: wst.DocString.Value, ContentType: wst.DocString.ContentType, Line: wst.DocString.Line}
	}
	if len(wst.Rows) > 0 {
		st.Rows = make([]Row, len(wst.Rows))
		for r, row := range wst.Rows {
			st.Rows[r] = Row(row)
		}
	}
	if len(wst.Embeddings) > 0 {
		st.Embeddings = make([]Embedding, len(wst.Embeddings))
		for e, we := range wst.Embeddings {
			if int64(len(we.Data)) > lim.Embedding {
				return Step{}, rel(ErrTooLarge, fmt.Sprintf(".embeddings[%d]", e))
			}
			st.Embeddings[e] = Embedding(we)
		}
	}
	return st, nil
}

func tags(wire []wireTag) []Tag {
	if len(wire) == 0 {
		return nil
	}
	out := make([]Tag, len(wire))
	for i, t := range wire {
		out[i] = Tag(t)
	}
	return out
}

// Validate checks a model built in code or by Parse: at least one feature, every feature has
// scenarios, every scenario has steps, every step has one of the six statuses and no duration is
// negative. A step whose result is entirely empty counts as a step without a status (ErrIncomplete).
func Validate(features []Feature) error {
	if len(features) == 0 {
		return ErrNoResults
	}
	for i, f := range features {
		if len(f.Elements) == 0 {
			return atPath(ErrIncomplete, "feature[%d]", i)
		}
		for j, sc := range f.Elements {
			if len(sc.Steps) == 0 {
				return atPath(ErrIncomplete, "feature[%d].elements[%d]", i, j)
			}
			for k, st := range sc.Steps {
				r := st.Result
				switch {
				case r.Status == "" && r.Duration == nil && r.Error == "":
					return atPath(ErrIncomplete, "feature[%d].elements[%d].steps[%d]", i, j, k)
				case !r.Status.Valid():
					return atPath(ErrInvalid, "feature[%d].elements[%d].steps[%d].result.status", i, j, k)
				case r.Duration != nil && *r.Duration < 0:
					return atPath(ErrInvalid, "feature[%d].elements[%d].steps[%d].result.duration", i, j, k)
				}
			}
		}
	}
	return nil
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
