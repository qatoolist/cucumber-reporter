package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	input "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

// F4-N3: same-stream monotonicity with every gate off, so only the outcome and
// runner-signal rules decide. Adding a non-passing case to a real capture must
// never make it greener; removing it again must restore the exact label, exit
// and counts.
func TestRound17SameStreamMonotonicityGatesOff(t *testing.T) {
	var files []string
	for _, pattern := range []string{
		"../testdata/round16/F3/cap/k*/run.*",
		"../testdata/captured/*/*strict*/run.*",
		"../testdata/round1[3-5]/captured*/*strict*/run.*",
	} {
		m, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range m {
			if strings.HasSuffix(f, ".json") || strings.HasSuffix(f, ".ndjson") {
				files = append(files, f)
			}
		}
	}
	if len(files) < 70 || !slicesContain(files, "k26-strict-pending-known-z/run.ndjson") {
		t.Fatalf("capture inventory changed: %d files", len(files))
	}
	rank := map[string]int{ToneOK: 0, ToneInfo: 1, ToneWarn: 2, ToneFail: 3}
	checks := 0
	for _, file := range files {
		base, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		format := "json"
		if strings.HasSuffix(file, ".ndjson") {
			format = "ndjson"
		}
		knownCases := []bool{false}
		if dir := filepath.Base(filepath.Dir(file)); strings.HasPrefix(dir, "k") && dir >= "k22" {
			knownCases = append(knownCases, true)
		}
		for _, known := range knownCases {
			opt := renderOptions{Version: 2}
			if known {
				opt.Annotations = []Annotation{{Scenario: "a-a8fde43a2452-L9", Kind: KindKnown, Ref: "K", Title: "known Z", Step: 1, StepText: "step z"}}
			}
			original, ok := monotonicOutcome(t, base, format, opt)
			if !ok {
				t.Fatalf("%s known=%v: real capture refused", file, known)
			}
			for _, statuses := range [][]string{{"pending"}, {"undefined"}, {"failed"}, {"failed", "passed"}} {
				name := fmt.Sprintf("%s known=%v %v", file, known, statuses)
				changed, ok := monotonicOutcome(t, injectRawCase(t, base, format, statuses), format, opt)
				if ok && (rank[changed.Tone] < rank[original.Tone] || changed.Exit < original.Exit) {
					t.Fatalf("%s: adding evidence improved %+v to %+v", name, original, changed)
				}
				restored, ok := monotonicOutcome(t, removeInjectedCase(t, injectRawCase(t, base, format, statuses), base, format), format, opt)
				if !ok || !reflect.DeepEqual(restored, original) {
					t.Fatalf("%s: removing injected evidence gave %+v, want %+v", name, restored, original)
				}
				checks++
			}
		}
	}
	if checks != 352 {
		t.Fatalf("property inventory changed: %d checks over %d captures", checks, len(files))
	}
}

type monotonicResult struct {
	Label, Tone  string
	Exit         int
	Counts       map[string]int
	GateFailures []string
	RunnerFailed bool
}

func monotonicOutcome(t *testing.T, raw []byte, format string, opt renderOptions) (monotonicResult, bool) {
	t.Helper()
	in := runInput{Name: "single"}
	var err error
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
	p, err := preparePublicRun([]runInput{in}, opt, RunConfig{})
	if err != nil {
		// A refusal is never greener than a verdict.
		return monotonicResult{}, false
	}
	exit := 0
	if len(p.Summary.GateFailures) > 0 {
		exit = 3
	}
	return monotonicResult{p.Summary.Computed.Label, p.Summary.Computed.Tone, exit, p.Summary.Counts, p.Summary.GateFailures, p.Summary.RunnerFailed}, true
}

func injectRawCase(t *testing.T, base []byte, format string, statuses []string) []byte {
	t.Helper()
	if format == "json" {
		var fs []any
		d := json.NewDecoder(bytes.NewReader(base))
		d.UseNumber()
		if err := d.Decode(&fs); err != nil {
			t.Fatal(err)
		}
		var elements []any
		for _, status := range statuses {
			elements = append(elements, map[string]any{"type": "scenario", "name": "Injected", "line": 3, "steps": []any{map[string]any{"name": "probe", "keyword": "Given ", "match": map[string]any{"location": "features/step_definitions/probe.rb:1"}, "result": map[string]any{"status": status}}}})
		}
		raw, err := json.Marshal(append(fs, map[string]any{"uri": "probe.feature", "name": "Probe", "elements": elements}))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	var b strings.Builder
	finish := ""
	for _, line := range strings.Split(strings.TrimRight(string(base), "\n"), "\n") {
		if strings.Contains(line, `"testRunFinished"`) {
			finish = line
			continue
		}
		b.WriteString(line + "\n")
	}
	b.WriteString(`{"pickle":{"id":"probe-p","uri":"probe.feature","name":"Injected","steps":[{"id":"probe-ps","text":"probe"}]}}` + "\n")
	b.WriteString(`{"testCase":{"id":"probe-c","pickleId":"probe-p","testSteps":[{"id":"probe-s","pickleStepId":"probe-ps"}]}}` + "\n")
	for i, status := range statuses {
		fmt.Fprintf(&b, "{\"testCaseStarted\":{\"id\":\"probe-%d\",\"testCaseId\":\"probe-c\",\"attempt\":%d,\"timestamp\":{\"seconds\":1}}}\n", i, i)
		fmt.Fprintf(&b, "{\"testStepFinished\":{\"testCaseStartedId\":\"probe-%d\",\"testStepId\":\"probe-s\",\"testStepResult\":{\"status\":%q,\"duration\":{\"seconds\":0}}}}\n", i, strings.ToUpper(status))
		fmt.Fprintf(&b, "{\"testCaseFinished\":{\"testCaseStartedId\":\"probe-%d\",\"timestamp\":{\"seconds\":2},\"willBeRetried\":%v}}\n", i, i < len(statuses)-1)
	}
	out := []byte(b.String())
	if finish != "" {
		out = append(out, finish+"\n"...)
		if statuses[len(statuses)-1] == "failed" {
			// A hard failure always fails the runner; soft statuses keep the
			// runner's own strictness decision.
			out = setRawSuccess(out, false)
		}
	}
	return out
}

// removeInjectedCase removes the probe from the changed artifact and restores
// the runner's original finish event.
func removeInjectedCase(t *testing.T, changed, base []byte, format string) []byte {
	t.Helper()
	if format == "json" {
		var fs []any
		d := json.NewDecoder(bytes.NewReader(changed))
		d.UseNumber()
		if err := d.Decode(&fs); err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(fs[:len(fs)-1])
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	var out bytes.Buffer
	for _, line := range bytes.Split(changed, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 || bytes.Contains(line, []byte(`"probe-`)) || bytes.Contains(line, []byte(`"testRunFinished"`)) {
			continue
		}
		out.Write(line)
		out.WriteByte('\n')
	}
	for _, line := range bytes.Split(base, []byte{'\n'}) {
		if bytes.Contains(line, []byte(`"testRunFinished"`)) {
			out.Write(line)
			out.WriteByte('\n')
		}
	}
	return out.Bytes()
}

func slicesContain(files []string, suffix string) bool {
	for _, f := range files {
		if strings.HasSuffix(filepath.ToSlash(f), suffix) {
			return true
		}
	}
	return false
}
