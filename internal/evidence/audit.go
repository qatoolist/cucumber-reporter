package evidence

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
)

// RunnerAudit is derived exclusively from envelope bytes. It contains no model
// metadata, report verdicts, annotations or runner-provided attempt counters.
// Its cases use arrival order and store statuses only, never evidence text.
type RunnerAudit struct {
	Failed     bool
	HookFailed bool
	Missing    bool
	Cases      [][]Execution
	Keys       []string
}

// Audit returns a value snapshot of the raw stream, independently of Check's
// parser correspondence check. No parsed model values enter this snapshot.
func (m *Messages) Audit() RunnerAudit {
	a := RunnerAudit{Failed: m.failed, HookFailed: m.hookFailed}
	keys := make([]string, 0, len(m.starts))
	for key := range m.starts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		ids := m.starts[key]
		p := m.pickles[m.cases[key]]
		own := 0
		for _, st := range p.Steps {
			background := false
			for _, id := range st.AST {
				background = background || m.background[id]
			}
			if !background {
				own++
			}
		}
		var attempts []Execution
		for _, id := range ids {
			e := m.results[id]
			e.NoOwnSteps = own == 0
			for _, step := range m.planned[key] {
				if !m.observed[id][step] {
					a.Missing = true
					e.MissingResults = true
				}
			}
			e.Results = append([]string(nil), e.Results...)
			for i := range e.Results {
				e.Results[i] = strings.ToLower(e.Results[i])
			}
			attempts = append(attempts, e)
		}
		a.Cases = append(a.Cases, attempts)
		line := 0
		for _, id := range p.AST {
			if m.lines[id] > 0 {
				line = m.lines[id]
			}
		}
		a.Keys = append(a.Keys, LocationKey(p.URI, line, p.Name))
	}
	return a
}

// Encode serializes only status evidence for transport through public value
// copies. It contains neither runner UUIDs nor paths or attachment contents.
func (a RunnerAudit) Encode() string { b, _ := json.Marshal(a); return string(b) }

// CheckRunner accounts for the raw failure signal by inventorying causes. A
// final hard failure can explain that signal, including an annotated failure.
// Soft/empty executions, rescued attempts, unreported retried hooks and failed
// run hooks cannot be explained by a hard failure elsewhere. Unlike production
// reconciliation this is a reduction of raw envelopes, with no Final/Stream or
// scenario metadata supplied by the production path.
func CheckRunner(encoded string, claimedFailure bool) error {
	if encoded == "" {
		return nil
	} // caller-constructed models have no raw artifact
	var a RunnerAudit
	if err := json.Unmarshal([]byte(encoded), &a); err != nil {
		return Floorf("evidence floor: invalid raw runner audit")
	}
	if !a.Failed && !a.HookFailed {
		return nil
	}
	causes, covered := 0, 0
	if a.HookFailed || a.Missing {
		causes++
	}
	for _, attempts := range a.Cases {
		if len(attempts) == 0 {
			causes++
			continue
		}
		last := attempts[len(attempts)-1]
		hard, soft := false, len(last.Results) == 0 || last.NoOwnSteps
		allPass := len(last.Results) > 0
		for _, s := range last.Results {
			hard = hard || s == "failed" || s == "ambiguous"
			soft = soft || s == "undefined" || s == "pending" || s == "unknown"
			allPass = allPass && s == "passed"
		}
		if hard {
			covered++
		}
		if soft {
			causes++
		}
		if len(attempts) > 1 && allPass {
			causes++
		} // strict-flaky may be the exit cause
		for _, prior := range attempts[:len(attempts)-1] {
			visibleFailure := false
			for _, s := range prior.Results {
				visibleFailure = visibleFailure || s != "passed"
			}
			if !visibleFailure {
				causes++
			} // Around failure omitted by runner
		}
	}
	if !claimedFailure && (causes > 0 || covered == 0) {
		return Floorf("evidence floor: raw runner failure is not accounted for; this is a reporter bug")
	}
	return nil
}

// LocationKey binds an observation to a location without retaining raw names or
// paths in the transported audit. It is computed independently of report IDs.
func LocationKey(uri string, line int, name string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d\x00%s", path.Clean(strings.ReplaceAll(uri, "\\", "/")), line, name))))
}

// CheckCases compares claimed per-scenario outcomes with executions collected
// directly from input bytes. No production retry/folding/status routine is used.
func CheckCases(encoded string, claimed map[string]Final) error {
	if encoded == "" {
		return nil
	}
	var audit RunnerAudit
	if err := json.Unmarshal([]byte(encoded), &audit); err != nil {
		return Floorf("evidence floor: invalid raw case audit")
	}
	for i, key := range audit.Keys {
		if key == "" {
			continue
		} // run hooks use the independent runner-signal check
		raw := audit.Cases[i]
		shown, ok := claimed[key]
		if !ok {
			return Floorf("evidence floor: raw scenario disappeared; this is a reporter bug")
		}
		if len(raw) == 0 {
			continue
		}
		mustStayRed := false
		for _, execution := range raw {
			for _, status := range execution.Results {
				mustStayRed = mustStayRed || status == "failed" || status == "ambiguous"
			}
		}
		final := raw[len(raw)-1]
		if (final.NoOwnSteps || final.MissingResults) && (shown.Status == "passed" || shown.Status == "known") && !hard(final) {
			return Floorf("evidence floor: incomplete raw execution became green; the parsed results were edited after parsing, or this is a reporter bug")
		}
		rescued := len(final.Results) > 0 && !final.NoOwnSteps
		for _, status := range final.Results {
			rescued = rescued && status == "passed"
		}
		if mustStayRed && !rescued && shown.Status != "failed" && shown.Status != "ambiguous" && shown.Status != "known" {
			return Floorf("evidence floor: raw failure has no passed rescue; the parsed results were edited after parsing, or this is a reporter bug")
		}
	}
	return nil
}

// LegacyAudit reads status records directly, grouping repeated locations in
// artifact order. Background records apply to the next scenario only. Text,
// durations, attachments and parser-specific metadata are deliberately ignored.
func LegacyAudit(raw []byte) (string, error) {
	type result struct {
		Status string
		Error  string `json:"error_message"`
	}
	type step struct {
		Result        result
		Before, After json.RawMessage
	}
	type element struct {
		Name, Type            string
		Line                  int
		Steps                 []json.RawMessage
		Before, After, Around json.RawMessage
	}
	var features []struct {
		URI      string
		Elements []element
	}
	if err := json.Unmarshal(raw, &features); err != nil {
		return "", err
	}
	audit := RunnerAudit{}
	indexes := map[string]int{}
	entries := func(raw json.RawMessage) []json.RawMessage {
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 {
			return nil
		}
		if raw[0] == '{' {
			return []json.RawMessage{raw}
		}
		var out []json.RawMessage
		if json.Unmarshal(raw, &out) != nil {
			return nil
		}
		return out
	}
	var collect func([]json.RawMessage) []string
	collect = func(values []json.RawMessage) []string {
		var out []string
		for _, raw := range values {
			var st step
			if json.Unmarshal(raw, &st) != nil {
				continue
			} // malformed optional passing metadata: legacy contract
			out = append(out, collect(entries(st.Before))...)
			status := strings.ToLower(st.Result.Status)
			if status == "" && st.Result.Error != "" {
				status = "failed"
			}
			if status != "" {
				out = append(out, status)
			}
			out = append(out, collect(entries(st.After))...)
		}
		return out
	}
	for _, f := range features {
		var background []string
		for _, sc := range f.Elements {
			results := collect(entries(sc.Before))
			results = append(results, collect(sc.Steps)...)
			results = append(results, collect(entries(sc.After))...)
			results = append(results, collect(entries(sc.Around))...)
			if strings.EqualFold(sc.Type, "background") {
				background = append(background, results...)
				continue
			}
			results = append(append([]string(nil), background...), results...)
			background = nil
			key := LocationKey(f.URI, sc.Line, sc.Name)
			index, ok := indexes[key]
			if !ok {
				index = len(audit.Cases)
				indexes[key] = index
				audit.Cases = append(audit.Cases, nil)
				audit.Keys = append(audit.Keys, key)
			}
			audit.Cases[index] = append(audit.Cases[index], Execution{Results: results, NoOwnSteps: len(sc.Steps) == 0})
		}
	}
	return audit.Encode(), nil
}

// CaseAudits partitions the raw inventory so a caller may explicitly select a
// subset of parsed scenarios without inheriting absent siblings' case assertions.
func CaseAudits(encoded string) (map[string]string, error) {
	var a RunnerAudit
	if err := json.Unmarshal([]byte(encoded), &a); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for i, key := range a.Keys {
		out[key] = (RunnerAudit{Keys: []string{key}, Cases: [][]Execution{a.Cases[i]}}).Encode()
	}
	return out, nil
}

// CheckCase checks the audit carried by one source scenario against that source's
// surviving execution. Location edits by a library caller do not rewrite evidence.
func CheckCase(encoded string, claimed Final, exists bool) error {
	if encoded == "" {
		return nil
	}
	if !exists {
		return Floorf("evidence floor: raw scenario disappeared; this is a reporter bug")
	}
	var audit RunnerAudit
	if err := json.Unmarshal([]byte(encoded), &audit); err != nil {
		return Floorf("evidence floor: invalid raw case audit")
	}
	shown := map[string]Final{}
	for _, key := range audit.Keys {
		shown[key] = claimed
	}
	return CheckCases(encoded, shown)
}
