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

	"github.com/qatoolist/cucumber-reporter/internal/evidence"

	modelmeta "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"
)

// ErrNoResults preserves the parser sentinel.
var (
	ErrNoResults = modelmeta.ErrNoResults
	// ErrInvalidJSON preserves the parser sentinel.
	ErrInvalidJSON = modelmeta.ErrInvalidJSON
	// ErrIncomplete preserves the parser sentinel.
	ErrIncomplete = modelmeta.ErrIncomplete
	// ErrInvalid preserves the parser sentinel.
	ErrInvalid = modelmeta.ErrInvalid
	// ErrTooLarge preserves the parser sentinel.
	ErrTooLarge = modelmeta.ErrTooLarge
	// ErrInvalidBase64 preserves the parser sentinel.
	ErrInvalidBase64 = modelmeta.ErrInvalidBase64
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
	wireArgument struct {
		Content     *string   `json:"content"`
		ContentType string    `json:"content_type"`
		MediaType   string    `json:"mediaType"`
		Rows        []wireRow `json:"rows"`
	}
	wireFeature struct {
		URI         string          `json:"uri"`
		Location    string          `json:"location"`
		ID          string          `json:"id"`
		Keyword     string          `json:"keyword"`
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Line        int             `json:"line"`
		Tags        []wireTag       `json:"tags"`
		Elements    *[]wireScenario `json:"elements"`
	}
	wireScenario struct {
		Around      json.RawMessage `json:"around"`
		Before      json.RawMessage `json:"before"`
		After       json.RawMessage `json:"after"`
		ID          string          `json:"id"`
		Keyword     string          `json:"keyword"`
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Line        int             `json:"line"`
		Type        string          `json:"type"`
		Tags        []wireTag       `json:"tags"`
		Steps       *[]wireStep     `json:"steps"`
	}
	wireTag struct {
		Name string `json:"name"`
		Line int    `json:"line"`
	}
	wireStep struct {
		Output      []string        `json:"output"`
		Arguments   []wireArgument  `json:"arguments"`
		Attachments []wireEmbedding `json:"attachments"`
		Hidden      json.RawMessage `json:"hidden"`
		Before      json.RawMessage `json:"before"`
		After       json.RawMessage `json:"after"`
		Keyword     string          `json:"keyword"`
		Name        string          `json:"name"`
		Line        int             `json:"line"`
		DocString   *wireDocString  `json:"doc_string"`
		Rows        []wireRow       `json:"rows"`
		Match       wireMatch       `json:"match"`
		Result      *wireResult     `json:"result"`
		Embeddings  []wireEmbedding `json:"embeddings"`
	}
	wireDocString struct {
		Value       string `json:"value"`
		ContentType string `json:"content_type"`
		MediaType   string `json:"media_type"`
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
// must be one non-empty array of complete features (every feature has scenarios and every step has one
// of the six statuses; a scenario without steps is kept and is undefined), with no duplicate object keys and nothing after
// the array. Unknown non-execution fields are ignored. Non-passing hooks and Background execution are retained.
// Attachment data is kept as given and checked when it is decoded.
func Parse(r io.Reader, lim Limits) ([]Feature, error) {
	lim = lim.WithDefaults()
	data, err := io.ReadAll(io.LimitReader(r, lim.Input+4))
	if err != nil {
		return nil, fmt.Errorf("cucumber: read input: %w", err)
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	if int64(len(data)) > lim.Input {
		return nil, fmt.Errorf("%w: input limit %d bytes", ErrTooLarge, lim.Input)
	}
	_, encodingErr := utf8Reader(bytes.NewReader(data))
	if encodingErr != nil {
		return nil, encodingErr
	}
	if err = checkSyntax(data); err != nil {
		return nil, err
	}
	var wire *[]wireFeature
	if looksLikeBehave(data) {
		return nil, fmt.Errorf("%w: behave JSON is not supported; export Cucumber JSON or Messages", ErrInvalidJSON)
	}
	if err = json.Unmarshal(data, &wire); err != nil {
		return nil, wireTypeError(err)
	}
	if wire == nil {
		return nil, ErrInvalidJSON
	}
	features, err := convert(*wire, lim)
	if err != nil {
		return nil, err
	}
	if err = Validate(features); err != nil {
		return nil, err
	}
	rawEvidence, scanErr := evidence.Scan(data)
	if scanErr != nil {
		return nil, ErrInvalidJSON
	}
	if err = checkRawScenarioEvidence(rawEvidence, features); err != nil {
		return nil, err
	}
	if err = evidence.Check(rawEvidence, modelmeta.Evidence(features)); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	audit, auditErr := evidence.LegacyAudit(data)
	if auditErr != nil {
		return nil, auditErr
	}
	attachRawCases(features, audit)
	return modelmeta.PlainFeatures(features)
}

// checkSyntax walks the token stream once. It requires a top-level array and rejects duplicate keys
// (compared the way encoding/json matches field names, ignoring case), excessive nesting and trailing data.
func checkSyntax(data []byte) error {
	if len(bytes.TrimSpace(data)) > 0 && bytes.TrimSpace(data)[0] == '{' && LooksLikeMessages(data) {
		return nilSafeMessagesHint()
	}
	if !json.Valid(data) {
		var raw json.RawMessage
		err := json.Unmarshal(data, &raw)
		return syntaxErrorAt(err, int64(len(data)))
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	first, err := dec.Token()
	if err != nil {
		return syntaxErrorAt(err, dec.InputOffset())
	}
	if d, ok := first.(json.Delim); !ok || d != '[' {
		if LooksLikeMessages(data) {
			return nilSafeMessagesHint()
		}
		return atOffset(ErrInvalidJSON, 0)
	}
	if err := walkContainer(dec, '[', 1); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return syntaxErrorAt(err, dec.InputOffset())
	}
	return nil
}

// walkContainer consumes the rest of an array or object whose opening delimiter has been read.
func walkContainer(dec *json.Decoder, open json.Delim, depth int) error {
	return walkKnownContainer(dec, open, depth, knownWireKey)
}

func walkKnownContainer(dec *json.Decoder, open json.Delim, depth int, known func(string) bool) error {
	if depth > maxDepth {
		return atOffset(fmt.Errorf("%w: nesting exceeds limit", ErrInvalidJSON), dec.InputOffset())
	}
	var seen map[string]struct{}
	if open == '{' {
		seen = map[string]struct{}{}
	}
	for dec.More() {
		if open == '{' {
			tok, err := dec.Token()
			if err != nil {
				return syntaxErrorAt(err, dec.InputOffset())
			}
			key, ok := tok.(string)
			if !ok {
				return syntaxErrorAt(err, dec.InputOffset())
			}
			folded := key
			if known(key) {
				folded = foldKey(key)
			}
			if _, dup := seen[folded]; dup {
				return atOffset(fmt.Errorf("%w: duplicate object key", ErrInvalidJSON), dec.InputOffset())
			}
			seen[folded] = struct{}{}
		}
		tok, err := dec.Token()
		if err != nil {
			return syntaxErrorAt(err, dec.InputOffset())
		}
		if d, ok := tok.(json.Delim); ok {
			if err := walkKnownContainer(dec, d, depth+1, known); err != nil {
				return err
			}
		}
	}
	if _, err := dec.Token(); err != nil { // closing delimiter
		return syntaxErrorAt(err, dec.InputOffset())
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
		if wf.URI == "" && wf.Location != "" {
			return nil, fmt.Errorf("%w: behave JSON is not supported; export Cucumber JSON or Messages", ErrInvalidJSON)
		}
		f := Feature{URI: wf.URI, ID: wf.ID, Keyword: wf.Keyword, Name: wf.Name, Description: wf.Description, Line: wf.Line, Tags: tags(wf.Tags)}
		if wf.Elements == nil {
			return nil, atPath(ErrIncomplete, "feature[%d]", i)
		}
		f.Elements = make([]Scenario, len(*wf.Elements))
		for j, ws := range *wf.Elements {
			sc := Scenario{ID: ws.ID, Keyword: ws.Keyword, Name: ws.Name, Description: ws.Description, Type: ws.Type, Line: ws.Line, Tags: tags(ws.Tags)}
			if sc.Type == "run_hook" {
				sc.Type = "scenario"
			} // Run hooks exist only in Messages.
			var wsteps []wireStep
			if ws.Steps != nil {
				wsteps = *ws.Steps
			}
			// Godog writes a scenario without steps with no "steps" key; it is kept, and is undefined.
			if len(wsteps) > 0 {
				sc.Steps = make([]Step, len(wsteps))
			}
			for k, wst := range wsteps {
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
			if err := attachHooks(&sc, ws, lim); err != nil {
				return nil, hookErrorAt(fmt.Sprintf("feature[%d].elements[%d]", i, j), err)
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
	if wst.DocString == nil {
		for _, arg := range wst.Arguments {
			if arg.Content != nil {
				typ := arg.ContentType
				if typ == "" {
					typ = arg.MediaType
				}
				wst.DocString = &wireDocString{Value: *arg.Content, ContentType: typ}
			}
			if len(arg.Rows) > 0 {
				wst.Rows = append(wst.Rows, arg.Rows...)
			}
		}
	}
	wst.Embeddings = append(wst.Embeddings, wst.Attachments...)
	if len(wst.Output) > 0 {
		wst.Embeddings = append(wst.Embeddings, wireEmbedding{Name: "Runner output", MimeType: "text/plain", Data: base64.StdEncoding.EncodeToString([]byte(strings.Join(wst.Output, "\n")))})
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
		if wst.DocString.ContentType == "" {
			wst.DocString.ContentType = wst.DocString.MediaType
		}
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
				return Step{}, rel(fmt.Errorf("%w: attachment limit %d base64 characters", ErrTooLarge, lim.Embedding), fmt.Sprintf(".embeddings[%d]", e))
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
// scenarios, every step has one of the six statuses and no duration is negative. A step whose result is entirely empty counts as a step without a status (ErrIncomplete).
func Validate(features []Feature) error {
	for i, f := range features {
		for j, sc := range f.Elements {
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

var knownJSONKeys = func() map[string]bool {
	keys := strings.Fields("uri id keyword name description line tags elements type steps before after around hidden doc_string content_type media_type rows cells match location result status error_message duration embeddings data output arguments attachments value start_timestamp")
	out := map[string]bool{}
	for _, key := range keys {
		out[foldKey(key)] = true
	}
	return out
}()

func knownWireKey(key string) bool { return knownJSONKeys[foldKey(key)] }

func looksLikeBehave(data []byte) bool {
	var features []struct {
		URI      string
		Location json.RawMessage
	}
	if json.Unmarshal(data, &features) != nil {
		return false
	}
	for _, f := range features {
		if f.URI == "" && len(f.Location) > 0 && f.Location[0] == '"' {
			return true
		}
	}
	return false
}

func checkMessageSyntax(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return ErrInvalidJSON
	}
	if d, ok := tok.(json.Delim); ok {
		return walkKnownContainer(dec, d, 0, func(key string) bool { return knownMessageKeys[foldKey(key)] })
	}
	return ErrInvalidJSON
}

func syntaxErrorAt(err error, fallback int64) error {
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		if syntax.Error() == "unexpected end of JSON input" {
			return atOffset(ErrInvalidJSON, fallback)
		}
		return atOffset(ErrInvalidJSON, syntax.Offset)
	}
	return atOffset(ErrInvalidJSON, fallback)
}

var knownMessageKeys = func() map[string]bool {
	keys := strings.Fields("meta source gherkinDocument pickle hook testCase testCaseStarted testStepStarted testStepFinished attachment testCaseFinished testRunStarted testRunFinished testRunHookStarted testRunHookFinished externalAttachment id uri name language feature children rule background scenario examples tableBody tableHeader cells value location line column keyword description tags steps astNodeIds text argument docString dataTable content mediaType testSteps pickleId hookId pickleStepId stepDefinitionIds testCaseId attempt workerId timestamp seconds nanos testCaseStartedId testStepId testStepResult status message duration exception type stackTrace cause willBeRetried success body contentEncoding fileName url testRunHookStartedId result git branch revision remote started finished")
	out := map[string]bool{}
	for _, key := range keys {
		out[foldKey(key)] = true
	}
	return out
}()

// LooksLikeMessages recognizes the envelope shape without echoing input values.
func LooksLikeMessages(data []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})))
	var envelope map[string]json.RawMessage
	if dec.Decode(&envelope) != nil {
		return false
	}
	for key := range envelope {
		switch key {
		case "meta", "source", "gherkinDocument", "pickle", "testCase", "testRunStarted", "testRunFinished":
			return true
		}
	}
	return false
}

type streamHint struct {
	message string
	cause   error
}

func (e *streamHint) Error() string { return e.message }
func (e *streamHint) Unwrap() error { return e.cause }
func nilSafeMessagesHint() error {
	return &streamHint{"Messages NDJSON input requires -enhanced -format messages (or -validate -format messages)", ErrInvalidJSON}
}

// Check each source element before folding, so moving a failure to another
// scenario cannot satisfy the raw-input floor merely by keeping run totals equal.
func checkRawScenarioEvidence(raw evidence.Counts, features []Feature) error {
	index := 0
	for _, f := range features {
		for _, sc := range f.Elements {
			if index >= len(raw.Scenarios) {
				return evidence.Floorf("%s:%d: evidence floor: source scenario mapping lost; this is a reporter bug", f.URI, sc.Line)
			}
			if err := evidence.Check(evidence.Counts{Results: raw.Scenarios[index]}, modelmeta.Evidence([]Feature{{Elements: []Scenario{sc}}})); err != nil {
				return fmt.Errorf("%s:%d: %w: non-passing result in an unsupported execution field", f.URI, sc.Line, ErrInvalid)
			}
			index++
		}
	}
	return nil
}

func attachRawCases(features []Feature, encoded string) {
	cases, _ := evidence.CaseAudits(encoded)
	for fi := range features {
		for si := range features[fi].Elements {
			sc := &features[fi].Elements[si]
			modelmeta.ScenarioMetadata(sc).RawCaseAudit = cases[evidence.LocationKey(features[fi].URI, sc.Line, sc.Name)]
		}
	}
}
