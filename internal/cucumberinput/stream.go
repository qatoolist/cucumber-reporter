package cucumber

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/qatoolist/cucumber-reporter/internal/evidence"

	modelmeta "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"
)

// ParseStream validates one feature at a time instead of retaining the entire input
// and wire model together. It retains the resulting features and preserves Parse's
// duplicate-key, depth, completeness, size and trailing-data checks.
func ParseStream(r io.Reader, limits Limits) ([]Feature, error) { return parseStream(r, limits, false) }

// ParseEnhanced preserves legacy scenario hooks for enhanced analysis and accepts
// empty shards, including scenario and step-local hook evidence.
func ParseEnhanced(r io.Reader, limits Limits) ([]Feature, error) {
	return parseStream(r, limits, true)
}

func parseStream(r io.Reader, limits Limits, enhanced bool) ([]Feature, error) {
	lim := limits.WithDefaults()
	reader, err := utf8Reader(r)
	if err != nil {
		return nil, err
	}
	bounded := &io.LimitedReader{R: reader, N: lim.Input + 1}
	dec := json.NewDecoder(bounded)
	first, err := dec.Token()
	if err != nil {
		return nil, streamSyntaxError(err, dec.InputOffset(), lim.Input+1-bounded.N)
	}
	if d, ok := first.(json.Delim); !ok || d != '[' {
		return nil, streamSyntaxError(err, dec.InputOffset(), lim.Input+1-bounded.N)
	}
	var out []Feature
	var lastFeatureEnd int64
	for dec.More() {
		var raw json.RawMessage
		if err = dec.Decode(&raw); err != nil {
			if bounded.N == 0 {
				return nil, fmt.Errorf("%w: input limit %d bytes", ErrTooLarge, lim.Input)
			}
			return nil, decoderSyntaxError(dec, err, lim.Input+1-bounded.N, len(out) > 0, lastFeatureEnd, bounded)
		}
		if bounded.N == 0 {
			return nil, fmt.Errorf("%w: input limit %d bytes", ErrTooLarge, lim.Input)
		}
		framed := make([]byte, 0, len(raw)+2)
		framed = append(framed, '[')
		framed = append(framed, raw...)
		framed = append(framed, ']')
		if err = checkSyntax(framed); err != nil {
			var pe *ParseError
			if errors.As(err, &pe) && pe.Offset >= 0 {
				pe.Offset += dec.InputOffset() - int64(len(raw)) - 1
			}
			return nil, err
		}
		if looksLikeBehave(framed) {
			return nil, fmt.Errorf("%w: behave JSON is not supported; export Cucumber JSON or Messages", ErrInvalidJSON)
		}
		rawEvidence, scanErr := evidence.Scan(framed)
		if scanErr != nil {
			return nil, ErrInvalidJSON
		}
		var wire wireFeature
		if err = json.Unmarshal(raw, &wire); err != nil {
			e := wireTypeError(err)
			var pe *ParseError
			if errors.As(e, &pe) && pe.Offset >= 0 {
				pe.Offset += dec.InputOffset() - int64(len(raw))
			}
			return nil, e
		}
		var features []Feature
		features, err = convert([]wireFeature{wire}, lim)
		if err != nil {
			var pe *ParseError
			if errors.As(err, &pe) {
				pe.Path = strings.Replace(pe.Path, "feature[0]", fmt.Sprintf("feature[%d]", len(out)), 1)
			}
			return nil, err
		}

		if err = checkRawScenarioEvidence(rawEvidence, features); err != nil {
			return nil, err
		}
		audit, auditErr := evidence.LegacyAudit(framed)
		if auditErr != nil {
			return nil, auditErr
		}
		attachRawCases(features, audit)
		represented := modelmeta.Evidence(features)
		if floorErr := evidence.Check(rawEvidence, represented); floorErr != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalid, floorErr)
		}
		if err = Validate(features); err != nil {
			return nil, err
		}
		out = append(out, features...)
		lastFeatureEnd = dec.InputOffset()
	}
	if _, err = dec.Token(); err != nil {
		return nil, streamSyntaxError(err, dec.InputOffset(), lim.Input+1-bounded.N)
	}
	tail := bufio.NewReader(io.MultiReader(dec.Buffered(), bounded))
	offset := dec.InputOffset()
	for {
		ch, width, readErr := tail.ReadRune()
		if bounded.N == 0 {
			return nil, fmt.Errorf("%w: input limit %d bytes", ErrTooLarge, lim.Input)
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
		offset += int64(width)
		if ch != ' ' && ch != '\n' && ch != '\r' && ch != '\t' {
			return nil, atOffset(ErrInvalidJSON, offset)
		}
	}

	if enhanced && len(out) == 0 {
		return out, nil
	}
	if err = Validate(out); err != nil {
		return nil, err
	}
	if !enhanced {
		return modelmeta.PlainFeatures(out)
	}
	return out, nil
}

func hookErrorAt(prefix string, err error) error {
	var pe *ParseError
	if errors.As(err, &pe) {
		return &ParseError{Path: prefix + pe.Path, Offset: -1, Err: pe.Err}
	}
	return &ParseError{Path: prefix, Offset: -1, Err: err}
}

func stepHooks(st *Step, wire wireStep, lim Limits) error {
	var err error
	st.Before, err = convertHooks(wire.Before, "before", lim)
	if err != nil {
		return hookErrorAt(".before", err)
	}
	st.After, err = convertHooks(wire.After, "after", lim)
	if err != nil {
		return hookErrorAt(".after", err)
	}
	return nil
}

func convertHooks(raw json.RawMessage, kind string, lim Limits) ([]Step, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		var single map[string]json.RawMessage
		if json.Unmarshal(raw, &single) != nil || single["result"] == nil {
			return nil, nil //nolint:nilerr // Legacy malformed hooks with no outcome carry no failure information.
		}
		entries = []json.RawMessage{raw}
	}
	var out []Step
	for i, entry := range entries {
		if sanitized, err := sanitizeHook(entry); err == nil {
			entry = sanitized
		}
		var probe struct {
			Result struct {
				Status string
				Error  string `json:"error_message"`
			}
		}
		_ = json.Unmarshal(entry, &probe)
		status := strings.ToLower(probe.Result.Status)
		if status == "" && probe.Result.Error == "" {
			status = "undefined"
		}
		var w wireStep
		if err := json.Unmarshal(entry, &w); err != nil {
			if status == "passed" && probe.Result.Error == "" {
				continue
			}
			return nil, hookErrorAt(fmt.Sprintf("[%d]", i), wireTypeError(err))
		}
		if w.Result == nil {
			w.Result = &wireResult{}
		}
		if status == "" && probe.Result.Error != "" {
			status = "failed"
		}
		if !Status(status).Valid() {
			if probe.Result.Error != "" {
				status = "failed"
			} else {
				status = "undefined"
			}
		}
		w.Result.Status = &status
		if status == "passed" && w.Result.Duration != nil && *w.Result.Duration < 0 {
			w.Result.Duration = nil
		}
		st, err := convertStep(w, lim)
		if err != nil {
			return nil, hookErrorAt(fmt.Sprintf("[%d]", i), err)
		}
		if err = stepHooks(&st, w, lim); err != nil {
			return nil, hookErrorAt(fmt.Sprintf("[%d]", i), err)
		}
		st.Keyword = "Hook "
		modelmeta.StepMetadata(&st).Origin = "hook"
		modelmeta.StepMetadata(&st).HookKind = kind
		if st.Name == "" {
			st.Name = kind + " hook"
		}
		out = append(out, st)
	}
	return out, nil
}

func attachHooks(sc *Scenario, ws wireScenario, lim Limits) error {
	var err error
	sc.Before, err = convertHooks(ws.Before, "before", lim)
	if err != nil {
		return hookErrorAt(".before", err)
	}
	sc.After, err = convertHooks(ws.After, "after", lim)
	if err != nil {
		return hookErrorAt(".after", err)
	}
	around, err := convertHooks(ws.Around, "around", lim)
	if err != nil {
		return hookErrorAt(".around", err)
	}
	sc.After = append(sc.After, around...)
	if ws.Steps != nil {
		for ti, wire := range *ws.Steps {
			hidden := false
			if len(wire.Hidden) > 0 && json.Unmarshal(wire.Hidden, &hidden) != nil {
				var text string
				_ = json.Unmarshal(wire.Hidden, &text)
				hidden = strings.EqualFold(text, "true")
			}
			keyword := strings.TrimSpace(wire.Keyword)
			if hidden || ((keyword == "Before" || keyword == "After") && strings.TrimSpace(wire.Name) == "") {
				modelmeta.StepMetadata(&sc.Steps[ti]).Origin = "hook"
			}
			if err = stepHooks(&sc.Steps[ti], wire, lim); err != nil {
				return hookErrorAt(fmt.Sprintf(".steps[%d]", ti), err)
			}
		}
	}
	return nil
}

// sanitizeHook retains the result while dropping malformed optional runner fields.
// It does not share the evidence scanner or determine the final verdict.
func sanitizeHook(entry json.RawMessage) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(entry, &fields); err != nil {
		return entry, err
	}
	clean := map[string]json.RawMessage{}
	for key, value := range fields {
		canonical := strings.ToLower(key)
		clean[canonical] = value
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(clean["result"], &result); err != nil {
		return entry, err
	}
	normalized := map[string]json.RawMessage{}
	for key, value := range result {
		normalized[strings.ToLower(key)] = value
	}
	result = normalized
	if value, ok := result["status"]; ok {
		var status string
		if json.Unmarshal(value, &status) != nil {
			result["status"] = json.RawMessage(`"undefined"`)
		}
	}
	if value, ok := result["duration"]; ok {
		var n int64
		if json.Unmarshal(value, &n) != nil || n < 0 {
			delete(result, "duration")
		}
	}
	if value, ok := result["error_message"]; ok {
		var text string
		if json.Unmarshal(value, &text) != nil {
			delete(result, "error_message")
		}
	}
	clean["result"], _ = json.Marshal(result)
	for _, key := range []string{"line", "name", "keyword", "match", "output", "doc_string", "rows", "embeddings", "attachments", "arguments"} {
		if value, ok := clean[key]; ok {
			probe := map[string]json.RawMessage{key: value}
			encoded, _ := json.Marshal(probe)
			var step wireStep
			if json.Unmarshal(encoded, &step) != nil {
				delete(clean, key)
			}
		}
	}
	return json.Marshal(clean)
}

func streamSyntaxError(err error, offset, read int64) error {
	var syntax *json.SyntaxError
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) || (errors.As(err, &syntax) && syntax.Error() == "unexpected end of JSON input") {
		offset = read
	} else if err != nil && !errors.As(err, &syntax) {
		offset++
	}
	return syntaxErrorAt(err, offset)
}

// Decoder.Decode after Token can diagnose an array separator rather than the
// offending value. Recheck its buffered fragment with a minimal array prefix;
// this preserves feature-at-a-time memory use and yields ordinary JSON offsets.
func decoderSyntaxError(dec *json.Decoder, original error, read int64, prior bool, lastFeatureEnd int64, bounded *io.LimitedReader) error {
	remainder, err := io.ReadAll(dec.Buffered())
	if err != nil {
		return streamSyntaxError(original, dec.InputOffset(), read)
	}
	// A decoder may stop in the middle of an invalid Unicode escape. A small
	// lookahead completes that token so offsets agree with whole-input decoding.
	inputLimit := read + bounded.N - 1
	extra, readErr := io.ReadAll(io.LimitReader(bounded, 16))
	if readErr != nil {
		return readErr
	}
	if bounded.N == 0 {
		return fmt.Errorf("%w: input limit %d bytes", ErrTooLarge, inputLimit)
	}
	remainder = append(remainder, extra...)
	read += int64(len(extra))
	prefix := []byte("[")
	if prior && read-int64(len(remainder)) <= lastFeatureEnd {
		prefix = []byte("[{}")
	}
	framed := append(prefix, remainder...)
	var raw json.RawMessage
	native := json.Unmarshal(framed, &raw)
	var syntax *json.SyntaxError
	if errors.As(native, &syntax) {
		if syntax.Error() == "unexpected end of JSON input" {
			return atOffset(ErrInvalidJSON, read)
		}
		return atOffset(ErrInvalidJSON, syntax.Offset+read-int64(len(remainder))-int64(len(prefix)))
	}
	return streamSyntaxError(original, dec.InputOffset(), read)
}
