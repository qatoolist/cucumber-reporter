package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	input "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

// Add cases to the SAME artifact, before parsing. These tests exercise the raw
// failure signal together with known annotations and rescued attempts. Removing
// the added case must recover the exact original result, not merely remain red.
func TestRound16SameStreamMonotonicity(t *testing.T) {
	checks := 0
	for _, format := range []string{"json", "ndjson"} {
		for _, known := range []bool{false, true} {
			base, err := os.ReadFile("../testdata/round16/F3/cap/k25-known-z-only/run." + format)
			if err != nil {
				t.Fatal(err)
			}
			if !known {
				base = removeRawCase(t, base, format, "Z")
			}
			opt := renderOptions{Version: 2}
			if known {
				opt.Annotations = []Annotation{{Scenario: "a-a8fde43a2452-L9", Kind: KindKnown, Ref: "K", Title: "known Z", Step: 1, StepText: "step z"}}
			}
			prepare := func(raw []byte) *preparedRun {
				t.Helper()
				in := runInput{Name: "single"}
				if format == "json" {
					in.Features, err = input.ParseEnhanced(bytes.NewReader(raw), input.Limits{})
				} else {
					var mr input.MessageRun
					mr, err = input.ParseMessages(bytes.NewReader(raw), input.Limits{})
					in.Attempts = mr.Attempts
				}
				if err != nil {
					t.Fatal(err)
				}
				p, e := preparePublicRun([]runInput{in}, opt, RunConfig{Gates: GatePolicy{Unexpected: true, Undefined: true, Pending: true, Flaky: true}})
				if e != nil {
					t.Fatal(e)
				}
				return p
			}
			original := prepare(base)
			if original.Summary.Computed.Tone == ToneFail {
				t.Fatal("monotonicity must start from a nonfailed run")
			}
			for _, statuses := range [][]string{{"pending"}, {"undefined"}, {"failed", "passed"}} {
				var raw []byte
				if format == "json" {
					var fs []map[string]any
					if err = json.Unmarshal(base, &fs); err != nil {
						t.Fatal(err)
					}
					var elements []any
					for _, status := range statuses {
						elements = append(elements, map[string]any{"type": "scenario", "name": "Injected", "line": 3, "steps": []any{map[string]any{"name": "probe", "keyword": "Given ", "match": map[string]any{"location": "features/step_definitions/probe.rb:1"}, "result": map[string]any{"status": status}}}})
					}
					fs = append(fs, map[string]any{"uri": "probe.feature", "name": "Probe", "elements": elements})
					raw, err = json.Marshal(fs)
					if err != nil {
						t.Fatal(err)
					}
				} else {
					var b strings.Builder
					for _, line := range strings.Split(string(base), "\n") {
						if !strings.Contains(line, `"testRunFinished"`) {
							b.WriteString(line + "\n")
						}
					}
					b.WriteString(`{"pickle":{"id":"probe-p","uri":"probe.feature","name":"Injected","steps":[{"id":"probe-ps","text":"probe"}]}}` + "\n")
					b.WriteString(`{"testCase":{"id":"probe-c","pickleId":"probe-p","testSteps":[{"id":"probe-s","pickleStepId":"probe-ps"}]}}` + "\n")
					for i, status := range statuses {
						fmt.Fprintf(&b, "{\"testCaseStarted\":{\"id\":\"probe-%d\",\"testCaseId\":\"probe-c\",\"attempt\":%d,\"timestamp\":{\"seconds\":1}}}\n", i, i)
						fmt.Fprintf(&b, "{\"testStepFinished\":{\"testCaseStartedId\":\"probe-%d\",\"testStepId\":\"probe-s\",\"testStepResult\":{\"status\":%q,\"duration\":{\"seconds\":0}}}}\n", i, strings.ToUpper(status))
						fmt.Fprintf(&b, "{\"testCaseFinished\":{\"testCaseStartedId\":\"probe-%d\",\"timestamp\":{\"seconds\":2}}}\n", i)
					}
					b.WriteString(`{"testRunFinished":{"success":false}}` + "\n")
					raw = []byte(b.String())
				}
				changed := prepare(raw)
				if changed.Summary.Computed.Tone != ToneFail || len(changed.Summary.GateFailures) == 0 {
					t.Fatalf("%s known=%v %v improved to %v", format, known, statuses, changed.Summary.Computed)
				}
				var removed []byte
				if len(statuses) == 2 {
					if format == "json" {
						var fs []map[string]any
						_ = json.Unmarshal(raw, &fs)
						last := fs[len(fs)-1]
						elements := last["elements"].([]any)
						last["elements"] = elements[1:]
						removed, _ = json.Marshal(fs)
					} else {
						removed = removeRawStart(raw, "probe-0", !known)
					}
				} else {
					removed = removeRawCase(t, raw, format, "Injected")
					if known && format == "ndjson" {
						removed = setRawSuccess(removed, false)
					}
				}
				restored := prepare(removed)
				if restored.Summary.Computed.Tone != original.Summary.Computed.Tone {
					t.Fatalf("removing injected evidence changed original verdict")
				}
				checks += 2
			}
		}
	}
	if checks != 24 {
		t.Fatalf("property inventory changed: %d", checks)
	}
}

func setRawSuccess(raw []byte, success bool) []byte {
	var out bytes.Buffer
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var event map[string]any
		_ = json.Unmarshal(line, &event)
		if finish, ok := event["testRunFinished"].(map[string]any); ok {
			finish["success"] = success
			line, _ = json.Marshal(event)
		}
		out.Write(line)
		out.WriteByte('\n')
	}
	return out.Bytes()
}

func removeRawStart(raw []byte, start string, success bool) []byte {
	var out bytes.Buffer
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if bytes.Contains(line, []byte(`"`+start+`"`)) {
			continue
		}
		out.Write(line)
		out.WriteByte('\n')
	}
	return setRawSuccess(out.Bytes(), success)
}

func removeRawCase(t *testing.T, raw []byte, format, name string) []byte {
	t.Helper()
	if format == "json" {
		var fs []map[string]any
		if err := json.Unmarshal(raw, &fs); err != nil {
			t.Fatal(err)
		}
		for _, f := range fs {
			kept := []any{}
			for _, el := range f["elements"].([]any) {
				if el.(map[string]any)["name"] != name {
					kept = append(kept, el)
				}
			}
			f["elements"] = kept
		}
		b, _ := json.Marshal(fs)
		return b
	}
	pickles, cases, starts := map[string]bool{}, map[string]bool{}, map[string]bool{}
	var events []map[string]any
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var e map[string]any
		if err := json.Unmarshal(line, &e); err != nil {
			t.Fatal(err)
		}
		events = append(events, e)
		if p, ok := e["pickle"].(map[string]any); ok && p["name"] == name {
			pickles[p["id"].(string)] = true
		}
	}
	for _, e := range events {
		if c, ok := e["testCase"].(map[string]any); ok && pickles[c["pickleId"].(string)] {
			cases[c["id"].(string)] = true
		}
	}
	for _, e := range events {
		if c, ok := e["testCaseStarted"].(map[string]any); ok && cases[c["testCaseId"].(string)] {
			starts[c["id"].(string)] = true
		}
	}
	var out bytes.Buffer
	for _, e := range events {
		drop := false
		for kind, value := range e {
			v, ok := value.(map[string]any)
			if !ok {
				continue
			}
			id, _ := v["testCaseStartedId"].(string)
			drop = drop || starts[id]
			if kind == "testCaseStarted" {
				id, _ := v["id"].(string)
				drop = drop || starts[id]
			}
		}
		if !drop {
			b, _ := json.Marshal(e)
			out.Write(b)
			out.WriteByte('\n')
		}
	}
	return setRawSuccess(out.Bytes(), true)
}
