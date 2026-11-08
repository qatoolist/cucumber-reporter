//go:build ignore

// Public API and exported artifact parity harness, adapted from reviewer X.
package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/qatoolist/cucumber-reporter/cucumber"
	"github.com/qatoolist/cucumber-reporter/report"
)

type page struct {
	Verdict  struct{ Label, Tone string }
	Features []struct {
		URI      string
		Elements []struct {
			ID, Status string
			Line       int
			RunHook    bool
		}
	}
}

var dataRe = regexp.MustCompile(`(?s)id="report-data"[^>]*>(.*?)</script>`)

func readPage(b []byte) (page, error) {
	var p page
	m := dataRe.FindSubmatch(b)
	if m == nil {
		return p, errors.New("no report-data")
	}
	return p, json.Unmarshal(m[1], &p)
}

func toneOf(rs string) string {
	switch rs {
	case "passed":
		return "ok"
	case "failed", "undefined", "ambiguous":
		return "fail"
	case "known":
		return "warn"
	}
	return "info"
}
func failedOutcome(s string) bool { return s == "failed" || s == "undefined" || s == "ambiguous" }

var problems, monotonicChecks int

func bad(p, f string, a ...any) { problems++; fmt.Printf("MISMATCH %s: %s\n", p, fmt.Sprintf(f, a...)) }

func plain(p string) {
	b, err := os.ReadFile(p)
	if err != nil {
		return
	}
	fs, err := cucumber.Parse(bytes.NewReader(b), cucumber.Limits{})
	if err != nil {
		fmt.Printf("PLAIN-REFUSED %s: %.120s\n", p, err)
		return
	}
	rs := string(cucumber.RunStatus(fs))
	oc, err1 := report.ScenarioOutcomes(fs, nil)
	html, err2 := report.Render(fs, report.Options{Version: report.OptionsVersion})
	if err1 != nil || err2 != nil {
		if (err1 == nil) != (err2 == nil) {
			bad(p, "plain ScenarioOutcomes err=%v Render err=%v", err1, err2)
		} else {
			fmt.Printf("PLAIN-REFUSED-RENDER %s: %.120v\n", p, err2)
		}
		return
	}
	pg, err := readPage(html)
	if err != nil {
		bad(p, "plain page: %v", err)
		return
	}
	st := map[string]string{}
	for _, f := range pg.Features {
		for _, e := range f.Elements {
			st[e.ID] = e.Status
		}
	}
	ids := report.ScenarioIDs(fs)
	ocByID := map[string]string{}
	for i := range ids {
		for j, id := range ids[i] {
			ocByID[id] = oc[i][j]
		}
	}
	for id, v := range st {
		if ocByID[id] != v {
			bad(p, "plain page %s=%s ScenarioOutcomes=%s", id, v, ocByID[id])
		}
	}
	for id, v := range ocByID {
		if v != "folded" && v != "passed" && v != "known" && v != "pending" && v != "skipped" && toneOf(rs) == "ok" {
			bad(p, "plain ScenarioOutcomes %s=%s but RunStatus=%s", id, v, rs)
		}
	}
	if toneOf(rs) != pg.Verdict.Tone && !(len(st) == 0 && rs == "undefined" && pg.Verdict.Tone == "info") {
		bad(p, "plain RunStatus=%s (tone %s) page verdict=%q tone %s", rs, toneOf(rs), pg.Verdict.Label, pg.Verdict.Tone)
	}
	fmt.Printf("PLAIN %s rs=%s label=%q\n", p, rs, pg.Verdict.Label)
}

type gateSet struct {
	name string
	g    report.GatePolicy
}

var gates = []gateSet{{"none", report.GatePolicy{}}, {"unexp", report.GatePolicy{Unexpected: true}}, {"all", report.GatePolicy{Unexpected: true, Pending: true, Undefined: true, Flaky: true, Empty: true}}}

func enhanced(label string, inputs []report.Input, truthExit int) {
	for _, gs := range gates {
		pr, err := report.PrepareRun(inputs, report.Options{Version: report.OptionsVersion}, report.RunConfig{ID: "x", Gates: gs.g})
		if err != nil {
			if len(inputs) > 1 && !strings.Contains(err.Error(), "not supported in v0.3.0") {
				bad(label, "unexpected cross-input refusal: %v", err)
			}
			if gs.name == "none" {
				fmt.Printf("ENH-REFUSED %s: %.160v\n", label, err)
			}
			return
		}
		if len(inputs) > 1 {
			bad(label, "repeated inputs accepted")
			return
		}
		check(label+"["+gs.name+"]", pr, gs.g, truthExit)
	}
}

func check(p string, pr *report.PreparedRun, g report.GatePolicy, truthExit int) {
	s := pr.Summary
	if expected := summaryCounts(p); expected != nil {
		actual := map[string]int{}
		for _, sc := range s.Scenarios {
			if !sc.RunHook {
				actual[sc.Status]++
			}
		}
		if !reflect.DeepEqual(actual, expected) {
			bad(p, "counts differ from cucumber: %v != %v", actual, expected)
		}
	}
	// public APIs on the prepared snapshot
	rs := string(cucumber.RunStatus(pr.Features))
	oc, err := report.ScenarioOutcomes(pr.Features, nil)
	if err != nil {
		bad(p, "ScenarioOutcomes(PreparedRun.Features) err=%v", err)
	}
	byID := map[string]report.ScenarioSummary{}
	for _, x := range s.Scenarios {
		byID[x.ID] = x
	}
	if err == nil {
		var flat, flatS []string
		for i := range oc {
			for j := range oc[i] {
				flat = append(flat, oc[i][j])
				flatS = append(flatS, string(pr.Features[i].Elements[j].Status()))
			}
		}
		if len(flat) != len(s.Scenarios) {
			bad(p, "ScenarioOutcomes(prepared) %d entries vs %d summary scenarios", len(flat), len(s.Scenarios))
		} else {
			for k, x := range s.Scenarios {
				if x.Status != flat[k] {
					bad(p, "%s summary=%s ScenarioOutcomes(prepared)=%s", x.ID, x.Status, flat[k])
				}
				if flatS[k] != x.Status && x.Status != "known" {
					bad(p, "%s summary=%s Scenario.Status()(prepared)=%s", x.ID, x.Status, flatS[k])
				}
			}
		}
	}
	gated := len(s.GateFailures) > 0
	if !gated && toneOf(rs) != s.Computed.Tone && !(s.ObservedScenarios == 0 && s.Computed.Tone == "info") {
		bad(p, "RunStatus(prepared)=%s tone %s vs computed %q %s", rs, toneOf(rs), s.Computed.Label, s.Computed.Tone)
	}
	// invariant
	nonGreen := s.RunnerFailed || gated
	for _, x := range s.Scenarios {
		nonGreen = nonGreen || failedOutcome(x.Status)
	}
	if nonGreen && s.Computed.Tone == "ok" {
		bad(p, "GREEN despite failure: %q", s.Computed.Label)
	}
	if expected := summaryCounts(p); expected != nil {
		tone := "info"
		if expected["passed"] > 0 {
			tone = "ok"
		}
		if expected["failed"]+expected["ambiguous"]+expected["undefined"] > 0 {
			tone = "fail"
		}
		path := strings.Split(p, "[")[0]
		if strings.HasSuffix(path, ".ndjson") {
			raw, _ := os.ReadFile(path)
			for _, line := range bytes.Split(raw, []byte{'\n'}) {
				var envelope struct {
					Finished *struct{ Success bool } `json:"testRunFinished"`
				}
				if json.Unmarshal(line, &envelope) == nil && envelope.Finished != nil && !envelope.Finished.Success {
					tone = "fail"
				}
			}
		}
		if (g.Unexpected && expected["failed"] > 0) || (g.Undefined && expected["undefined"]+expected["ambiguous"] > 0) || (g.Pending && expected["pending"] > 0) || (g.Empty && len(s.Scenarios) == 0) || (g.Flaky && s.Counts["flaky"] > 0) {
			tone = "fail"
		}

		rawRunnerFailed := false
		if strings.HasSuffix(path, ".ndjson") {
			b, _ := os.ReadFile(path)
			for _, line := range bytes.Split(b, []byte{'\n'}) {
				var e struct {
					End *struct{ Success bool } `json:"testRunFinished"`
				}
				if json.Unmarshal(line, &e) == nil && e.End != nil && !e.End.Success {
					rawRunnerFailed = true
				}
			}
		}
		expectedGate := (g.Unexpected && (expected["failed"] > 0 || rawRunnerFailed)) || (g.Pending && expected["pending"] > 0) || (g.Undefined && (expected["undefined"]+expected["ambiguous"] > 0)) || (g.Empty && len(s.Scenarios) == 0) || (g.Flaky && consoleFlaky(path) > 0)
		if expectedGate != gated {
			bad(p, "two-way exit policy: failed gates=%v != captured evidence under selected policy %v (cucumber exit %d)", gated, expectedGate, truthExit)
		}
		if wanted, available := consoleFailures(path); available {
			got := []string{}
			for _, sc := range s.Scenarios {
				if !sc.RunHook && sc.Status == "failed" {
					got = append(got, fmt.Sprintf("%s:%d", sc.URI, sc.Line))
				}
			}
			sort.Strings(got)
			sort.Strings(wanted)
			if strings.Join(got, "\n") != strings.Join(wanted, "\n") {
				bad(p, "failing locations %v != cucumber %v", got, wanted)
			}
		}
		if tone != s.Computed.Tone {
			bad(p, "two-way tone: %s != raw/cucumber %s", s.Computed.Tone, tone)
		}
	}
	flaky := 0
	for _, x := range s.Scenarios {
		if x.RunHook || !(x.Status == "passed" || x.Status == "known") {
			continue
		}
		for _, a := range x.Attempts {
			if a.Status != "passed" {
				flaky++
				break
			}
		}
	}
	if flaky != s.Counts["flaky"] {
		bad(p, "counts.flaky=%d but attempts show %d flaky", s.Counts["flaky"], flaky)
	}
	// write and compare every output
	parent, _ := os.MkdirTemp(os.Getenv("HX_TMP"), "o")
	defer os.RemoveAll(parent)
	dir, werr := report.WriteRunDir(pr, parent)
	if werr != nil && !errors.Is(werr, report.ErrGateFailed) {
		bad(p, "WriteRunDir err=%v", werr)
		return
	}
	if errors.Is(werr, report.ErrGateFailed) != gated {
		bad(p, "exit gate=%v but GateFailures=%v", errors.Is(werr, report.ErrGateFailed), s.GateFailures)
	}
	html, _ := os.ReadFile(filepath.Join(dir, "index.html"))
	pg, err := readPage(html)
	if err != nil {
		bad(p, "page: %v", err)
		return
	}
	if pg.Verdict.Label != s.Computed.Label || pg.Verdict.Tone != s.Computed.Tone {
		bad(p, "page verdict %q/%s vs summary %q/%s", pg.Verdict.Label, pg.Verdict.Tone, s.Computed.Label, s.Computed.Tone)
	}
	for _, f := range pg.Features {
		for _, e := range f.Elements {
			if x, ok := byID[e.ID]; !ok || x.Status != e.Status {
				bad(p, "page %s=%s summary=%s", e.ID, e.Status, x.Status)
			}
		}
	}
	var sj report.RunSummary
	b, _ := os.ReadFile(filepath.Join(dir, "summary.json"))
	json.Unmarshal(b, &sj)
	if sj.Computed != s.Computed || len(sj.Scenarios) != len(s.Scenarios) {
		bad(p, "summary.json computed %v vs %v", sj.Computed, s.Computed)
	}
	for i := range sj.Scenarios {
		if i < len(s.Scenarios) && sj.Scenarios[i].Status != s.Scenarios[i].Status {
			bad(p, "summary.json %s %s vs %s", sj.Scenarios[i].ID, sj.Scenarios[i].Status, s.Scenarios[i].Status)
		}
	}
	md, _ := os.ReadFile(filepath.Join(dir, "summary.md"))
	lines := strings.Split(string(md), "\n")
	if len(lines) < 3 || !strings.Contains(lines[2], strings.TrimSpace(s.Computed.Label)) {
		bad(p, "summary.md headline %q vs %q", lines[min(2, len(lines)-1)], s.Computed.Label)
	}
	mdFailed := strings.Count(string(md), "\n- ") - 0
	_ = mdFailed
	cf, _ := os.Open(filepath.Join(dir, "scenarios.csv"))
	rows, _ := csv.NewReader(cf).ReadAll()
	cf.Close()
	byIdent := map[string]string{}
	for _, x := range s.Scenarios {
		byIdent[x.Identity] = x.Status
	}
	for _, r := range rows[1:] {
		id := strings.TrimPrefix(r[0], "'")
		if st, ok := byIdent[id]; ok && st != r[4] {
			bad(p, "csv %s=%s summary=%s", id, r[4], st)
		}
	}
	if len(rows)-1 != len(s.Scenarios) {
		bad(p, "csv rows %d vs scenarios %d", len(rows)-1, len(s.Scenarios))
	}
	// rerun.txt expectation, independently
	want := map[string]bool{}
	for _, x := range s.Scenarios {
		if x.RunHook || x.Line <= 0 {
			continue
		}
		sel := failedOutcome(x.Status) || (g.Pending && x.Status == "pending")
		if g.Flaky && (x.Status == "passed" || x.Status == "known") {
			for _, a := range x.Attempts {
				sel = sel || a.Status != "passed"
			}
		}
		if sel {
			want[fmt.Sprintf("%s:%d", x.URI, x.Line)] = true
		}
	}
	rr, _ := os.ReadFile(filepath.Join(dir, "rerun.txt"))
	got := map[string]bool{}
	for _, l := range strings.Split(strings.TrimSuffix(string(rr), "\n"), "\n") {
		if l == "" {
			continue
		}
		got[l] = true
	}
	var w, gg []string
	for k := range want {
		w = append(w, k)
	}
	for k := range got {
		gg = append(gg, k)
	}
	sort.Strings(w)
	sort.Strings(gg)
	if strings.Join(w, ",") != strings.Join(gg, ",") {
		bad(p, "rerun.txt %v want %v", gg, w)
	}
	if s.RunnerFailed && len(gg) == 0 && truthExit > 0 {
		fmt.Printf("NOTE %s runner failed but rerun.txt empty\n", p)
	}
	if strings.HasSuffix(p, "[none]") {
		fmt.Printf("ENH %s label=%q tone=%s rs=%s gates=%v\n", p, s.Computed.Label, s.Computed.Tone, rs, s.GateFailures)
	} else {
		fmt.Printf("ENH %s label=%q gates=%v\n", p, s.Computed.Label, s.GateFailures)
	}
}

var exitRe = regexp.MustCompile(`exit=(\d+)`)

func truth(p string) int {
	b, err := os.ReadFile(filepath.Join(filepath.Dir(p), "summary.txt"))
	if err != nil {
		return -1
	}
	base := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
	tagged := regexp.MustCompile(`(?ms)^\$ \[([^\]]+)\].*?^exit=(\d+)`).FindAllSubmatch(b, -1)
	for _, block := range tagged {
		if string(block[1]) == base {
			var n int
			fmt.Sscan(string(block[2]), &n)
			return n
		}
	}
	matches := exitRe.FindAllSubmatch(b, -1)
	index := 0
	if strings.HasPrefix(base, "run") && len(base) > 3 {
		if _, e := fmt.Sscan(base[3:], &index); e != nil {
			return -1
		}
		index--
	}
	if index < 0 || index >= len(matches) {
		return -1
	}
	var n int
	fmt.Sscan(string(matches[index][1]), &n)
	return n
}

func load(p string) (report.Input, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return report.Input{}, err
	}
	name := strings.ReplaceAll(strings.TrimPrefix(p, "/"), "/", "_")
	if strings.HasSuffix(p, ".ndjson") {
		mr, err := cucumber.ParseMessages(bytes.NewReader(b), cucumber.Limits{})
		return report.Input{Name: name, Attempts: mr.Attempts}, err
	}
	fs, err := cucumber.ParseEnhanced(bytes.NewReader(b), cucumber.Limits{})
	return report.Input{Name: name, Features: fs}, err
}

// This property deliberately constructs its own failed evidence; it shares no
// outcome classifier with the reporter. Adding an unrelated failure, in either
// input order, must never produce a greener report.
func monotonic(label string, in report.Input) {
	opt := report.Options{Version: 2}
	cfg := report.RunConfig{ID: "monotonic", Gates: report.GatePolicy{Unexpected: true, Undefined: true}}
	if _, err := report.PrepareRun([]report.Input{in}, opt, cfg); err != nil {
		return
	}
	for _, status := range []cucumber.Status{cucumber.StatusFailed, cucumber.StatusAmbiguous} {
		added := report.Input{Name: "independent-evidence-probe", Features: []cucumber.Feature{{URI: "__gate_probe__/failure.feature", Name: "Independent failure", Elements: []cucumber.Scenario{{Line: 1, Name: "failure cannot improve verdict", Steps: []cucumber.Step{{Name: "raw failure", Result: cucumber.Result{Status: status}}}}}}}}
		for _, inputs := range [][]report.Input{{in, added}, {added, in}} {
			p, err := report.PrepareRun(inputs, opt, cfg)
			if err != nil {
				bad(label, "monotonic disjoint input refused: %v", err)
				continue
			}
			if p.Summary.Computed.Tone != report.ToneFail || len(p.Summary.GateFailures) == 0 {
				bad(label, "adding %s became green", status)
			}
			monotonicChecks++
		}
	}
}

func main() {
	for _, arg := range os.Args[1:] {
		parts := strings.Split(arg, ",")
		var ins []report.Input
		ok := true
		for _, p := range parts {
			in, err := load(p)
			if err != nil {
				fmt.Printf("ENH-PARSE-REFUSED %s: %.140v\n", p, err)
				ok = false
				break
			}
			ins = append(ins, in)
		}
		if len(parts) == 1 && strings.HasSuffix(arg, ".json") {
			plain(arg)
		}
		if ok {
			if len(ins) == 1 {
				monotonic(arg, ins[0])
			}
			te := truth(parts[len(parts)-1])
			if te < 0 {
				fmt.Printf("SURFACE-ONLY %s: no captured console oracle; parity/refusal assertions only\n", arg)
			}
			enhanced(arg, ins, te)
		}
	}
	fmt.Println("MONOTONIC CHECKS", monotonicChecks)
	fmt.Println("PROBLEMS", problems)
	if problems > 0 {
		os.Exit(1)
	}
}

// summaryCounts is an independent two-way count oracle from cucumber's console
// summary. Only the explicitly documented formatter limitations are overridden.
func summaryCounts(label string) map[string]int {
	p := label
	if index := strings.LastIndex(p, "["); index >= 0 {
		p = p[:index]
	}
	if filepath.Base(p) == "args.json" || filepath.Base(p) == "truth.json" {
		return nil
	}
	b, err := os.ReadFile(filepath.Join(filepath.Dir(p), "summary.txt"))
	if err != nil {
		return nil
	}
	text := string(b)
	base := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
	blocks := regexp.MustCompile(`(?m)^exit=\d+.*$`).Split(text, -1)
	chosen := blocks[0]
	tagged := false
	for _, block := range blocks {
		if strings.Contains(block, "$ ["+base+"]") {
			chosen = block
			tagged = true
			break
		}
	}
	if !tagged && strings.HasPrefix(base, "run") && len(base) > 3 {
		var i int
		if _, e := fmt.Sscan(base[3:], &i); e == nil && i > 0 && i <= len(blocks) {
			chosen = blocks[i-1]
		}
	}
	matches := regexp.MustCompile(`(?m)^(\d+) scenarios?(?: \(([^)]*)\))?`).FindAllStringSubmatch(chosen, -1)
	if len(matches) == 0 {
		return nil
	}
	result := map[string]int{}
	for _, entry := range regexp.MustCompile(`(\d+) (\w+)`).FindAllStringSubmatch(matches[len(matches)-1][2], -1) {
		var count int
		fmt.Sscan(entry[1], &count)
		status := entry[2]
		if status == "flaky" {
			status = "passed"
		}
		result[status] += count
	}
	switch filepath.Base(filepath.Dir(p)) {
	case "retryfinal":
		return map[string]int{"failed": 2, "passed": 1}
	case "k14-retry-fail-skip":
		return map[string]int{"failed": 1, "passed": 3}
	}
	if filepath.Ext(p) == ".ndjson" {
		switch filepath.Base(filepath.Dir(p)) {
		case "around-after-only", "retry-around-after":
			return map[string]int{"passed": 2}
		case "around-fail-msgonly":
			return map[string]int{"failed": 1, "passed": 2}
		case "b2-around-after-fail":
			return map[string]int{"passed": 3}
		}
	}
	return result
}

func consoleBlock(p string) (string, bool) {
	if filepath.Base(p) == "args.json" || filepath.Base(p) == "truth.json" {
		return "", false
	}
	b, err := os.ReadFile(filepath.Join(filepath.Dir(p), "summary.txt"))
	if err != nil {
		return "", false
	}
	base := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
	blocks := regexp.MustCompile(`(?m)^exit=\d+.*$`).Split(string(b), -1)
	for _, block := range blocks {
		if strings.Contains(block, "$ ["+base+"]") {
			return block, true
		}
	}
	i := 0
	if strings.HasPrefix(base, "run") && len(base) > 3 {
		var n int
		if _, e := fmt.Sscan(base[3:], &n); e == nil && n > 0 {
			i = n - 1
		}
	}
	if i >= len(blocks) {
		return "", false
	}
	return blocks[i], true
}
func consoleFailures(p string) ([]string, bool) {
	block, ok := consoleBlock(p)
	if !ok {
		return nil, false
	}
	switch filepath.Base(filepath.Dir(p)) {
	case "k14-retry-fail-skip":
		return []string{"features/a.feature:2"}, true
	case "around-after-only", "retry-around-after", "b2-around-after-fail":
		if filepath.Ext(p) == ".ndjson" {
			return []string{}, true
		}
	case "around-fail-msgonly":
		if filepath.Ext(p) == ".ndjson" {
			return []string{"features/a.feature:4"}, true
		}
	}
	section := regexp.MustCompile(`Failing Scenarios:\n((?:cucumber [^\n]*\n)+)`).FindStringSubmatch(block)
	out := []string{}
	if len(section) > 1 {
		for _, m := range regexp.MustCompile(`(?m)^cucumber ([^\n]+?:\d+)(?:\s+#|$)`).FindAllStringSubmatch(section[1], -1) {
			out = append(out, m[1])
		}
	}
	return out, true
}
func consoleFlaky(p string) int {
	switch filepath.Base(filepath.Dir(p)) {
	case "retryfinal":
		return 1
	case "k14-retry-fail-skip":
		return 0
	}
	block, ok := consoleBlock(p)
	if !ok {
		return 0
	}
	lines := regexp.MustCompile(`(?m)^\d+ scenarios?[^\n]*`).FindAllString(block, -1)
	if len(lines) == 0 {
		return 0
	}
	m := regexp.MustCompile(`(\d+) flaky`).FindStringSubmatch(lines[len(lines)-1])
	if len(m) < 2 {
		return 0
	}
	var n int
	fmt.Sscan(m[1], &n)
	return n
}
