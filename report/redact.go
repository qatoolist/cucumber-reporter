package report

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"

	modelmeta "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"
	"github.com/qatoolist/cucumber-reporter/internal/textsafe"

	cucumber "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

// Redaction applies explicit patterns, JSON keys, line-based headers and attachment exclusions.
// No automatic secret detection is claimed. ExcludeAttachments uses regular expressions on names.
type Redaction struct {
	Patterns           []string `json:"patterns,omitempty"`
	JSONKeys           []string `json:"jsonKeys,omitempty"`
	Headers            []string `json:"headers,omitempty"`
	ExcludeAttachments []string `json:"excludeAttachments,omitempty"`
}

type redactor struct {
	patterns, exclude []*regexp.Regexp
	keys              map[string]bool
	headers           []*regexp.Regexp
	headerKeys        map[string]bool
}

func newRedactor(p *Redaction) (*redactor, error) {
	r := &redactor{keys: map[string]bool{}, headerKeys: map[string]bool{}}
	for _, s := range p.Patterns {
		re, err := regexp.Compile(s)
		if err != nil {
			return nil, invalid("redaction pattern")
		}
		r.patterns = append(r.patterns, re)
	}
	for _, s := range p.Headers {
		s = strings.TrimSpace(s)
		if s == "" {
			return nil, invalid("empty redaction header")
		}
		r.headers = append(r.headers, regexp.MustCompile(`(?im)(?:^|[^A-Za-z0-9-])['"]?`+regexp.QuoteMeta(s)+`['"]?[ \t]*:[ \t]*([^\r\n]*)`))
		r.headers = append(r.headers, regexp.MustCompile(`(?i)"`+regexp.QuoteMeta(s)+`"[ \t\r\n]*:[ \t\r\n]*"((?:\\.|[^"\\\r\n])*)(?:"|$|[\r\n])`))
		r.headerKeys[strings.ToLower(s)] = true
	}
	for _, s := range p.JSONKeys {
		r.keys[strings.ToLower(s)] = true
	}
	for _, s := range p.ExcludeAttachments {
		re, err := regexp.Compile(s)
		if err != nil {
			return nil, invalid("attachment exclusion pattern")
		}
		r.exclude = append(r.exclude, re)
	}
	return r, nil
}

const redacted = "[REDACTED]"

func (r *redactor) text(s string) string { return string(r.textBytes([]byte(s))) }

// textBytes masks s. Display output deletes SGR coloring and browsers do not draw
// zero-width characters, so text either side of them reads as adjacent and can
// rejoin a secret the raw bytes split. When matching the visible text finds more
// to mask, the visible text is returned with the union of both matches masked.
func (r *redactor) textBytes(s []byte) []byte {
	out := r.maskBytes(s)
	hidden := textsafe.Hidden(s)
	if len(hidden) == 0 {
		return out
	}
	// at maps each raw offset to its offset in the visible text.
	visible := make([]byte, 0, len(s))
	at := make([]int, len(s)+1)
	h := 0
	for i := 0; i <= len(s); i++ {
		at[i] = len(visible)
		if i == len(s) {
			break
		}
		for h < len(hidden) && hidden[h][1] <= i {
			h++
		}
		if h < len(hidden) && hidden[h][0] <= i {
			continue
		}
		visible = append(visible, s[i])
	}
	found := r.spans(visible)
	if len(found) == 0 {
		return out
	}
	var mapped [][2]int
	for _, sp := range r.spans(s) {
		if a, b := at[sp[0]], at[sp[1]]; a < b {
			mapped = append(mapped, [2]int{a, b})
		}
	}
	// Keep the raw result, hidden bytes included, unless the visible matches mask
	// something the raw matches leave readable.
	masked := r.apply(visible, append(found, mapped...))
	if bytes.Equal(masked, r.apply(visible, mapped)) {
		return out
	}
	return masked
}

func (r *redactor) maskBytes(s []byte) []byte { return r.apply(s, r.spans(s)) }

func (r *redactor) spans(s []byte) [][2]int {
	var spans [][2]int
	for _, p := range r.patterns {
		for _, m := range p.FindAllIndex(s, -1) {
			if m[0] != m[1] {
				spans = append(spans, [2]int{m[0], m[1]})
			}
		}
	}
	for _, p := range r.headers {
		for _, m := range p.FindAllSubmatchIndex(s, -1) {
			if m[2] != m[3] {
				spans = append(spans, [2]int{m[2], m[3]})
			}
		}
	}
	return spans
}

func (r *redactor) apply(s []byte, spans [][2]int) []byte {
	if len(spans) == 0 {
		return s
	}
	spans = slices.Clone(spans)
	for offset := 0; offset < len(s); {
		i := bytes.Index(s[offset:], []byte(redacted))
		if i < 0 {
			break
		}
		i += offset
		spans = append(spans, [2]int{i, i + len(redacted)})
		offset = i + len(redacted)
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i][0] < spans[j][0] })
	var out bytes.Buffer
	cursor := 0
	for i := 0; i < len(spans); {
		start, end := spans[i][0], spans[i][1]
		i++
		for i < len(spans) && spans[i][0] < end {
			end = max(end, spans[i][1])
			i++
		}
		out.Write(s[cursor:start])
		out.WriteString(redacted)
		cursor = end
	}
	out.Write(s[cursor:])
	return out.Bytes()
}

// ErrRedaction means evidence could not satisfy the configured redaction policy.
var ErrRedaction = errors.New("report: redaction failed")

type redactionError struct{ location, message string }

func (e *redactionError) Error() string {
	if e.location == "" {
		return ErrRedaction.Error() + ": " + e.message
	}
	return ErrRedaction.Error() + " at " + e.location + ": " + e.message
}
func (e *redactionError) Unwrap() error { return ErrRedaction }
func redactionAt(location string, err error) error {
	var e *redactionError
	if errors.As(err, &e) {
		if e.location != "" {
			location += "." + e.location
		}
		return &redactionError{location: location, message: e.message}
	}
	return &redactionError{location: location, message: "cannot inspect evidence"}
}

func emptyRedaction(p *Redaction) bool {
	return p == nil || len(p.Patterns)+len(p.JSONKeys)+len(p.Headers)+len(p.ExcludeAttachments) == 0
}

// strings redacts display strings and explicit imported identities. Computed
// identifiers, structural enums and binary data are preserved.
func (r *redactor) strings(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			r.strings(v.Elem())
		}
	case reflect.Struct:
		// Doc strings have already been processed as evidence, including textual
		// values. Do not scan serialized JSON again as plain display text.
		if v.Type() == reflect.TypeFor[cucumber.DocString]() {
			return
		}
		for i := 0; i < v.NumField(); i++ {
			if !v.Field(i).CanInterface() {
				continue
			}
			name := v.Type().Field(i).Name
			if oneOf(name, "ReportID", "Origin", "Status", "Kind", "Type", "Tone", "Role", "State", "MimeType", "Data", "Started", "Finished", "RecordedAt", "Expires", "ReviewDate", "Change", "Completeness", "VerdictSource", "TimingSection", "Defaults") ||
				(name == "Identity" && (v.Type() == reflect.TypeFor[ScenarioMeta]() || ((v.Type() == reflect.TypeFor[ScenarioSummary]() || v.Type() == reflect.TypeFor[IssueObservation]()) && v.FieldByName("GeneratedIdentity").Bool()))) ||
				(name == "URI" && ((v.Type() == reflect.TypeFor[cucumber.Feature]() && syntheticFeature(v)) || (v.Type() == reflect.TypeFor[ScenarioSummary]() && v.FieldByName("SyntheticURI").Bool()))) ||
				(name == "ID" && ((v.Type() == reflect.TypeFor[RunSummary]() || v.Type() == reflect.TypeFor[HistoryPoint]()) && v.FieldByName("GeneratedID").Bool())) ||
				(name == "ID" && (v.Type() == reflect.TypeFor[ScenarioSummary]() || v.Type() == reflect.TypeFor[Section]())) ||
				(v.Type() == reflect.TypeFor[Annotation]() && name == "Scenario") ||
				v.Type() == reflect.TypeFor[Comparison]() {
				continue
			}
			r.strings(v.Field(i))
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			r.strings(v.Index(i))
		}
	case reflect.String:
		if v.CanSet() {
			v.SetString(r.text(v.String()))
		}
	}
}

// jsonValue edits token spans in place, preserving order, whitespace, duplicate
// keys and the exact spelling/precision of untouched values. Changed non-string
// scalars become strings so that the redacted document remains valid JSON.
func (r *redactor) jsonValue(raw json.RawMessage) (json.RawMessage, error) {
	// Empty response bodies contain no values to mask (for example HTTP 204).
	if len(bytes.Trim(raw, " \t\r\n")) == 0 {
		return raw, nil
	}
	if !json.Valid(raw) {
		// Structural key rules cannot safely inspect incomplete JSON. Never publish
		// evidence with a configured policy silently skipped, even if patterns match.
		if len(r.keys) > 0 {
			return nil, &redactionError{message: "cannot apply jsonKeys to malformed JSON"}
		}
		return []byte(r.text(string(raw))), nil
	}
	var out bytes.Buffer
	cursor := 0
	for i := 0; i < len(raw); {
		if strings.ContainsRune(" \t\r\n{}[]:,", rune(raw[i])) {
			i++
			continue
		}
		start := i
		var value string
		isString := raw[i] == '"'
		if isString {
			i++
			for raw[i] != '"' {
				if raw[i] == '\\' {
					i++
				}
				i++
			}
			i++
			if err := json.Unmarshal(raw[start:i], &value); err != nil {
				return nil, err
			}
		} else {
			for i < len(raw) && !strings.ContainsRune(" \t\r\n,]}", rune(raw[i])) {
				i++
			}
			value = string(raw[start:i])
		}
		if replacement := r.text(value); replacement != value {
			encoded, err := json.Marshal(replacement)
			if err != nil {
				return nil, err
			}
			out.Write(raw[cursor:start])
			out.Write(encoded)
			cursor = i
		}
		// A string followed by a colon is an object key. Match JSONKeys against
		// its original decoded spelling, even when a pattern also masks the key.
		next := i
		for next < len(raw) && strings.ContainsRune(" \t\r\n", rune(raw[next])) {
			next++
		}
		if isString && next < len(raw) && raw[next] == ':' && (r.keys[visibleKey(value)] || r.headerKeys[visibleKey(value)]) {
			next++
			for strings.ContainsRune(" \t\r\n", rune(raw[next])) {
				next++
			}
			d := json.NewDecoder(bytes.NewReader(raw[next:]))
			var hidden json.RawMessage
			if err := d.Decode(&hidden); err != nil {
				return nil, err
			}
			out.Write(raw[cursor:next])
			out.WriteString(`"[REDACTED]"`)
			i = next + int(d.InputOffset())
			cursor = i
		}
	}
	out.Write(raw[cursor:])
	return out.Bytes(), nil
}

// Only known binary formats bypass body scanning. Unknown media types are text
// candidates, and even invalid UTF-8 retains byte-pattern protection.
func binaryAttachment(mt string) bool {
	if strings.HasSuffix(mt, "+xml") {
		return false
	}
	for _, prefix := range []string{"image/", "audio/", "video/", "font/", "application/vnd.openxmlformats-officedocument."} {
		if strings.HasPrefix(mt, prefix) {
			return true
		}
	}
	return oneOf(mt, "application/pdf", "application/zip", "application/octet-stream", "application/gzip", "application/x-gzip", "application/x-tar", "application/x-bzip2", "application/x-xz", "application/x-7z-compressed", "application/vnd.rar", "application/wasm", "application/cbor", "application/protobuf", "application/x-protobuf", "application/msword")
}

func jsonEvidence(mt string, raw []byte) bool {
	if isJSON(mt) {
		return true
	}
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') && json.Valid(raw)
}

func isJSONLines(mt string) bool {
	return oneOf(mt, "application/ndjson", "application/x-ndjson", "text/x-ndjson", "ndjson",
		"application/jsonl", "application/jsonlines", "application/x-jsonlines", "jsonl", "jsonlines")
}

func (r *redactor) attachmentText(raw []byte, mt string) ([]byte, error) {
	if binaryAttachment(mt) {
		return raw, nil
	}
	return r.textEvidence(raw, mt)
}

func (r *redactor) textEvidence(raw []byte, mt string) ([]byte, error) {
	if jsonEvidence(mt, raw) {
		return r.jsonValue(raw)
	}
	if isJSONLines(mt) {
		lines := bytes.Split(raw, []byte{'\n'})
		for i, line := range lines {
			// Empty records, including the segment after a trailing newline, carry
			// no values and are safe to retain unchanged.
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}
			var err error
			lines[i], err = r.jsonValue(line)
			if err != nil {
				return nil, redactionAt(fmt.Sprintf("line[%d]", i), err)
			}
		}
		return bytes.Join(lines, []byte{'\n'}), nil
	}
	return r.textBytes(raw), nil
}

func (r *redactor) attachments(steps []cucumber.Step) error {
	for i := range steps {
		st := &steps[i]
		var kept []cucumber.Embedding
		for ai, a := range st.Embeddings {
			excluded := false
			for _, re := range r.exclude {
				excluded = excluded || re.MatchString(a.Name)
			}
			if excluded {
				continue
			}
			raw, err := a.Decode(int64(len(a.Data)))
			if err != nil {
				return &redactionError{location: fmt.Sprintf("steps[%d].embeddings[%d]", i, ai), message: "cannot inspect malformed attachment"}
			}
			raw, err = r.attachmentText(raw, mediaType(a.MimeType))
			if err != nil {
				return redactionAt(fmt.Sprintf("steps[%d].embeddings[%d]", i, ai), err)
			}
			a.Data = base64.StdEncoding.EncodeToString(raw)
			a.Name = r.text(a.Name)
			kept = append(kept, a)
		}
		st.Embeddings = kept
		if st.DocString != nil {
			doc := st.DocString
			raw := []byte(doc.Value)
			mt := mediaType(doc.ContentType)
			if jsonEvidence(mt, raw) || isJSONLines(mt) {
				out, err := r.textEvidence(raw, mt)
				if err != nil {
					return redactionAt(fmt.Sprintf("steps[%d].doc_string", i), err)
				}
				doc.Value = string(out)
			} else {
				doc.Value = r.text(doc.Value)
			}
			doc.ContentType = r.text(doc.ContentType)
		}
	}
	return nil
}

func redactRun(features []cucumber.Feature, opt *renderOptions, metadata map[string]ScenarioMeta, explicit map[string]bool, p *Redaction) error {
	if emptyRedaction(p) {
		return nil
	}
	r, err := newRedactor(p)
	if err != nil {
		return err
	}
	// Clone options because callers own slices and nested structures.
	analysis, bundle, execution := opt.Analysis, opt.BundleAttachments, opt.execution
	if analysis != nil {
		raw, cloneErr := json.Marshal(analysis)
		if cloneErr != nil {
			return cloneErr
		}
		analysis = nil
		if cloneErr = json.Unmarshal(raw, &analysis); cloneErr != nil {
			return cloneErr
		}
	}
	b, err := json.Marshal(opt)
	if err != nil {
		return err
	}
	*opt = renderOptions{}
	if err = json.Unmarshal(b, opt); err != nil {
		return err
	}
	opt.Analysis, opt.BundleAttachments, opt.execution = analysis, bundle, execution
	ids := scenarioIDs(features)
	locations := map[string]string{}
	for fi := range features {
		for si := range features[fi].Elements {
			location := fmt.Sprintf("feature[%d].elements[%d]", fi, si)
			locations[ids[fi][si]] = location
			if err = r.attachments(features[fi].Elements[si].Steps); err != nil {
				return redactionAt(location, err)
			}
		}
	}
	r.strings(reflect.ValueOf(features))
	r.strings(reflect.ValueOf(opt))
	for id, m := range metadata {
		for i := range m.Attempts {
			steps := internalSteps(m.Attempts[i].Steps, 0)
			if err = r.attachments(steps); err != nil {
				return redactionAt(fmt.Sprintf("%s.attempts[%d]", locations[id], i), err)
			}
			m.Attempts[i].Steps = publicSteps(steps, 0)
		}
		if explicit[id] {
			m.Identity = r.text(m.Identity)
		}
		r.strings(reflect.ValueOf(&m))
		metadata[id] = m
	}

	// Public snapshots retain private provenance for API/render parity. Replace
	// historical model bodies with the already-redacted attempt evidence before
	// those sidecars can be reused by a caller without the prepared Options.
	for fi := range features {
		for si := range features[fi].Elements {
			sc := &features[fi].Elements[si]
			state := modelmeta.ScenarioMetadata(sc)
			state.AttemptBackground = nil // current Steps already contain the folded execution
			if len(state.PreviousAttempts) > 0 {
				var previous []cucumber.Scenario
				attempts := metadata[ids[fi][si]].Attempts
				if len(attempts) < 2 {
					return fmt.Errorf("redaction: feature[%d].elements[%d] is missing retry evidence", fi, si)
				}
				for _, a := range attempts[:len(attempts)-1] {
					prior := cucumber.Scenario{Type: sc.Type, Name: sc.Name, Line: sc.Line, Steps: internalSteps(a.Steps, 0)}
					modelmeta.ScenarioMetadata(&prior).EnhancedFolded = true
					previous = append(previous, prior)
				}
				state.PreviousAttempts = previous
			}
		}
	}
	return nil
}

func redactSummaries(s *RunSummary, h []RunSummary, p *Redaction) error {
	r, err := newRedactor(p)
	if err != nil {
		return err
	}
	r.strings(reflect.ValueOf(s))
	r.strings(reflect.ValueOf(h))
	return nil
}

// ValidateInput reports unsupported result-bearing fields with safe JSON paths.
// Parse remains the authority for validity; this optional inspection never prints values.
func validateInput(data []byte, lim cucumber.Limits) ([]Diagnostic, error) {
	if _, err := cucumber.ParseEnhanced(bytes.NewReader(data), lim); err != nil {
		return nil, err
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	var decoded any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	var out []Diagnostic
	var walk func(any, string)
	walk = func(v any, p string) {
		switch x := v.(type) {
		case []any:
			for i, v := range x {
				walk(v, fmt.Sprintf("%s[%d]", p, i))
			}
		case map[string]any:
			for k, v := range x {
				switch k {
				case "before", "after", "around":
					entries, ok := v.([]any)
					if !ok && v != nil {
						out = append(out, Diagnostic{Path: p + "." + k, Message: "Ignored malformed hook metadata"})
					}
					for i, entry := range entries {
						m, _ := entry.(map[string]any)
						r, _ := m["result"].(map[string]any)
						status, statusString := r["status"].(string)
						if r["status"] != nil && !statusString {
							out = append(out, Diagnostic{Path: fmt.Sprintf("%s.%s[%d].result.status", p, k, i), Message: "Non-string hook status retained as undefined"})
						}
						malformed := false
						for _, key := range []string{"duration", "error_message"} {
							if value, ok := r[key]; ok && value != nil {
								if key == "error_message" {
									_, valid := value.(string)
									malformed = malformed || !valid
								} else {
									n, valid := value.(json.Number)
									if !valid {
										malformed = true
									} else {
										v, e := n.Int64()
										malformed = malformed || e != nil || v < 0
									}
								}
							}
						}
						for _, key := range []string{"line", "output", "match"} {
							if value, ok := m[key]; ok && value != nil {
								switch key {
								case "line":
									n, valid := value.(json.Number)
									_, e := n.Int64()
									malformed = malformed || !valid || e != nil
								case "output":
									_, valid := value.([]any)
									malformed = malformed || !valid
								case "match":
									_, valid := value.(map[string]any)
									malformed = malformed || !valid
								}
							}
						}
						if malformed {
							out = append(out, Diagnostic{Path: fmt.Sprintf("%s.%s[%d]", p, k, i), Message: "Malformed optional hook metadata ignored; outcome retained"})
						}
						if status != "" && !cucumber.Status(strings.ToLower(status)).Valid() {
							out = append(out, Diagnostic{Path: fmt.Sprintf("%s.%s[%d].result.status", p, k, i), Message: "Unknown hook status is represented as failed when error evidence exists, otherwise undefined"})
						}
						if r["status"] == nil {
							out = append(out, Diagnostic{Path: fmt.Sprintf("%s.%s[%d]", p, k, i), Message: "Hook with no outcome retained as undefined"})
						}
					}
				case "output":
					if !strings.Contains(p, ".steps[") && !strings.Contains(p, ".before[") && !strings.Contains(p, ".after[") && !strings.Contains(p, ".around[") {
						out = append(out, Diagnostic{Path: p + ".output", Message: "Unsupported execution field: output outside a step or hook"})
					}
				case "hooks", "retry", "attempt":
					out = append(out, Diagnostic{Path: p, Message: "Unsupported execution field: " + k})
				} // Never echo arbitrary field names.
				if oneOf(k, "elements", "steps", "before", "after", "around") {
					walk(v, p+"."+k)
				}
			}
		}
	}
	walk(decoded, "feature")
	if features, ok := decoded.([]any); ok {
		for i, v := range features {
			f, _ := v.(map[string]any)
			elements, _ := f["elements"].([]any)
			if len(elements) > 0 {
				last, _ := elements[len(elements)-1].(map[string]any)
				if last["type"] == "background" {
					out = append(out, Diagnostic{Path: fmt.Sprintf("feature[%d].elements[%d]", i, len(elements)-1), Message: "Orphan Background: non-passing results are retained; passing results are omitted"})
				}
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return naturalPathLess(out[i].Path, out[j].Path) })
	return out, nil
}

func syntheticFeature(v reflect.Value) bool {
	f := v.Interface().(cucumber.Feature)
	return modelmeta.FeatureMetadata(&f).SyntheticRunHooks
}

// Compare numeric array components numerically, without including evidence text.
func naturalPathLess(a, b string) bool {
	for len(a) > 0 && len(b) > 0 {
		if a[0] >= '0' && a[0] <= '9' && b[0] >= '0' && b[0] <= '9' {
			i, j := 0, 0
			for i < len(a) && a[i] >= '0' && a[i] <= '9' {
				i++
			}
			for j < len(b) && b[j] >= '0' && b[j] <= '9' {
				j++
			}
			if i != j {
				return i < j
			}
			if a[:i] != b[:j] {
				return a[:i] < b[:j]
			}
			a = a[i:]
			b = b[j:]
		} else {
			if a[0] != b[0] {
				return a[0] < b[0]
			}
			a = a[1:]
			b = b[1:]
		}
	}
	return len(a) < len(b)
}

// visibleKey is a JSON key as a reader sees it: hidden characters removed, so a
// zero-width character cannot split a configured key name.
func visibleKey(k string) string {
	b := []byte(k)
	hidden := textsafe.Hidden(b)
	for i := len(hidden) - 1; i >= 0; i-- {
		b = append(b[:hidden[i][0]:hidden[i][0]], b[hidden[i][1]:]...)
	}
	return strings.ToLower(string(b))
}
