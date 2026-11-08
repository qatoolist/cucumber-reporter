package report

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pub "github.com/qatoolist/cucumber-reporter/cucumber"
	input "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
	model "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"
)

func round14Input(t *testing.T, name, file string) runInput {
	t.Helper()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	in := runInput{Name: name}
	if strings.HasSuffix(file, ".ndjson") {
		m, e := pub.ParseMessages(bytes.NewReader(b), pub.Limits{})
		err = e
		in.Attempts = internalAttempts(m.Attempts)
	} else {
		in.Features, err = input.ParseEnhanced(bytes.NewReader(b), input.Limits{})
	}
	if err != nil {
		t.Fatal(err)
	}
	return in
}

func TestRound14ShardsNeverRescueEachOther(t *testing.T) {
	var ins []runInput
	for _, name := range []string{"chrome", "firefox"} {
		ins = append(ins, round14Input(t, name, "../testdata/round14/shards/"+name+"/cucumber.json"))
	}
	p, err := preparePublicRun(ins, renderOptions{Version: 2}, RunConfig{Gates: GatePolicy{Unexpected: true}})
	if err == nil && p.Summary.Computed.Tone != ToneFail {
		t.Fatalf("cross-shard rescue: %+v", p.Summary.Computed)
	}
	if err != nil && !strings.Contains(err.Error(), "reruns") {
		t.Fatal(err)
	}
}

func TestRound14AliasesTargetOnlyLiveIDs(t *testing.T) {
	roots := []string{"../testdata/captured", "../testdata/round13", "../testdata/round14"}
	for _, root := range roots {
		err := filepath.WalkDir(root, func(file string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(file, ".json") {
				return nil
			}
			b, _ := os.ReadFile(file)
			f, e := input.ParseEnhanced(bytes.NewReader(b), input.Limits{})
			if e != nil {
				return nil //nolint:nilerr // Corpus also includes options and deliberately invalid inputs.
			}
			for _, enhanced := range []bool{false, true} {
				opt, _ := normalize(renderOptions{Version: 2})
				features := f
				if enhanced {
					p, e := preparePublicRun([]runInput{{Name: "one", Features: f}}, opt, RunConfig{})
					if e != nil {
						continue
					}
					features, opt = p.Features, p.Options
				}
				page, e := buildPage(features, opt, nil)
				if e != nil {
					continue
				}
				live := map[string]bool{}
				for _, f := range page.Features {
					for _, s := range f.Elements {
						live[s.ID] = true
					}
				}
				for from, to := range page.Aliases {
					if live[from] || !live[to] {
						t.Errorf("%s enhanced=%v: invalid alias %s -> %s", file, enhanced, from, to)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestRound14MessagesFloorIncludesAllAttempts(t *testing.T) {
	in := round14Input(t, "one", "../testdata/captured/ruby-9.2.1-r12/retryfinal/run.ndjson")
	merged, _, err := mergeInputAttempts([]runInput{in})
	if err != nil {
		t.Fatal(err)
	}
	caught := 0
	for _, f := range merged[0].Features {
		for _, s := range f.Elements {
			if s.Status() == input.StatusFailed && model.ExecutionStatus(s) != input.StatusFailed {
				if model.CheckOutcome(s, string(model.ExecutionStatus(s)), nil) == nil {
					t.Fatal("Messages retry floor accepted a green mutant")
				}
				caught++
			}
		}
	}
	if caught == 0 {
		t.Fatal("fixture did not exercise failed retry promotion")
	}
}

func TestRound14AroundEvidenceAccepted(t *testing.T) {
	in := round14Input(t, "one", "../testdata/round14/captured/b5-around-attach-log/run.ndjson")
	p, err := preparePublicRun([]runInput{in}, renderOptions{Version: 2}, RunConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = renderPublicModel(p.Features, p.Options); err != nil {
		t.Fatal(err)
	}
}

func TestRound14CrossInputRerunsRefused(t *testing.T) {
	entries, e := os.ReadDir("../testdata/round14/captured")
	if e != nil {
		t.Fatal(e)
	}
	for _, d := range entries {
		base := "../testdata/round14/captured/" + d.Name()
		if _, e := os.Stat(base + "/run2.json"); e != nil {
			continue
		}
		for _, format := range []string{"json", "ndjson"} {
			var ins []runInput
			for n := 1; ; n++ {
				file := fmt.Sprintf("%s/run%d.%s", base, n, format)
				if _, e := os.Stat(file); e != nil {
					break
				}
				ins = append(ins, round14Input(t, fmt.Sprintf("run%d", n), file))
			}
			if _, e := preparePublicRun(ins, renderOptions{Version: 2}, RunConfig{}); e == nil || !strings.Contains(e.Error(), "not supported in v0.3.0") {
				t.Fatalf("%s %s: %v", d.Name(), format, e)
			}
		}
	}
}

func TestRound14FlakyAroundGate(t *testing.T) {
	for _, format := range []string{"json", "ndjson"} {
		in := round14Input(t, "one", "../testdata/round14/captured/b4b-around-after-fail-retry/run."+format)
		p, e := preparePublicRun([]runInput{in}, renderOptions{Version: 2}, RunConfig{Gates: GatePolicy{Flaky: true}})
		if e != nil {
			t.Fatal(e)
		}
		if p.Summary.Counts["flaky"] != 1 || len(p.Summary.GateFailures) == 0 {
			t.Fatalf("%s: %+v", format, p.Summary)
		}
	}
}

func TestRound14KnownRetries(t *testing.T) {
	for _, name := range []string{"c3-known-only", "c5-known-retry-flaky-then-known"} {
		for _, format := range []string{"json", "ndjson"} {
			in := round14Input(t, "one", "../testdata/round14/captured/"+name+"/run."+format)
			p, e := preparePublicRun([]runInput{in}, renderOptions{Version: 2}, RunConfig{})
			if e != nil {
				t.Fatal(e)
			}
			var ann []Annotation
			for fi, f := range p.Features {
				for si, s := range f.Elements {
					for _, st := range s.Steps {
						if model.StepMetadata(&st).Origin == "own" && st.Result.Status == input.StatusFailed {
							ann = append(ann, Annotation{Scenario: scenarioIDs(p.Features)[fi][si], Kind: KindKnown, Step: model.StepMetadata(&st).OwnIndex, StepText: st.Name, Title: "Known bug"})
							break
						}
					}
				}
			}
			p, e = preparePublicRun([]runInput{in}, renderOptions{Version: 2, Annotations: ann}, RunConfig{Gates: GatePolicy{Unexpected: true}})
			if e != nil {
				t.Fatal(e)
			}
			if p.Summary.Counts["known"] == 0 || len(p.Summary.GateFailures) > 0 {
				t.Fatalf("%s: %+v", format, p.Summary)
			}
		}
		in := round14Input(t, "plain", "../testdata/round14/captured/"+name+"/run.json")
		id := ""
		for fi, f := range in.Features {
			for si, sc := range f.Elements {
				if sc.Name == "Known one" {
					id = scenarioIDs(in.Features)[fi][si]
				}
			}
		}
		opt, err := normalize(renderOptions{Version: 2, Annotations: []Annotation{{Scenario: id, Kind: KindKnown, Step: 2, StepText: `outcome "K1"`, Title: "Known bug"}}})
		if err != nil {
			t.Fatal(err)
		}
		page, err := buildPage(in.Features, opt, nil)
		if err != nil {
			t.Fatal(err)
		}
		if page.Verdict.Tone != ToneWarn {
			t.Fatalf("plain %s: %+v", name, page.Verdict)
		}
	}
}

func TestRound14BundleSpilledEvidence(t *testing.T) {
	data := strings.Repeat("sample evidence\n", 230000)
	encoded := base64.StdEncoding.EncodeToString([]byte(data))
	sc := input.Scenario{Name: "Evidence budget", Line: 1, Steps: []input.Step{{Name: "output", Result: input.Result{Status: input.StatusPassed}}}}
	for i := 0; i < 12; i++ {
		sc.Steps[0].Embeddings = append(sc.Steps[0].Embeddings, input.Embedding{Name: fmt.Sprintf("log-%d.txt", i), MimeType: "text/plain", Data: encoded})
	}
	p, e := preparePublicRun([]runInput{{Name: "one", Features: []input.Feature{{URI: "budget.feature", Elements: []input.Scenario{sc}}}}}, renderOptions{Version: 2}, RunConfig{Bundle: true})
	if e != nil {
		t.Fatal(e)
	}
	dir, e := writeRunDir(p, t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	z, e := zip.OpenReader(filepath.Join(dir, "report.zip"))
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	sidecars := 0
	for _, f := range z.File {
		if strings.HasPrefix(f.Name, "attachment-") {
			r, e := f.Open()
			if e != nil {
				t.Fatal(e)
			}
			b, e := io.ReadAll(r)
			_ = r.Close()
			if e != nil || string(b) != data {
				t.Fatal("sidecar payload changed")
			}
			sidecars++
		}
	}
	if sidecars == 0 {
		t.Fatal("budget did not spill")
	}
}

func TestRound14BackgroundAnnotationMigration(t *testing.T) {
	f := []input.Feature{{URI: "bg.feature", Elements: []input.Scenario{
		{Type: "background", Line: 2, Steps: []input.Step{{Name: "setup", Result: input.Result{Status: input.StatusFailed}}}},
		{Type: "scenario", Name: "scenario", Line: 4, Steps: []input.Step{{Name: "action", Result: input.Result{Status: input.StatusSkipped}}}},
	}}}
	for _, enhanced := range []bool{false, true} {
		opt := renderOptions{Version: 2, Annotations: []Annotation{{Scenario: "bg-L4", Kind: KindKnown, BackgroundStep: 1, StepText: "setup", Title: "Known setup defect"}}}
		if enhanced {
			p, e := preparePublicRun([]runInput{{Name: "one", Features: f}}, opt, RunConfig{Gates: GatePolicy{Unexpected: true}})
			if e != nil {
				t.Fatal(e)
			}
			if p.Summary.Counts["known"] != 1 || len(p.Summary.GateFailures) != 0 {
				t.Fatal(p.Summary.Computed)
			}
		} else {
			opt, _ = normalize(opt)
			p, e := buildPage(f, opt, nil)
			if e != nil {
				t.Fatal(e)
			}
			if p.Features[0].Elements[0].Status != "known" {
				t.Fatal(p.Verdict)
			}
		}
	}
}

func TestRound14MessagesHookOriginsAndNames(t *testing.T) {
	for _, name := range []string{"hooks-fail", "retry-hooks"} {
		var want []string
		for _, format := range []string{"json", "ndjson"} {
			in := round14Input(t, "one", "../testdata/round13/captured-ruby-9.2.1/"+name+"/run."+format)
			p, err := preparePublicRun([]runInput{in}, renderOptions{Version: 2}, RunConfig{})
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, f := range p.Features {
				for _, sc := range f.Elements {
					for i, st := range sc.Steps {
						meta := model.StepMetadata(&st)
						if meta.Origin == "hook" {
							got = append(got, sc.Name+":"+stepLocation(sc, i+1))
							for _, a := range st.Embeddings {
								if !strings.HasPrefix(a.Name, "Before hook ") && !strings.HasPrefix(a.Name, "After hook ") {
									t.Fatalf("%s: unnamed hook evidence %q", format, a.Name)
								}
							}
						}
					}
				}
			}
			if format == "json" {
				want = got
			} else {
				if len(want) != len(got) {
					t.Fatalf("%s: lost hook rows", name)
				}
				for i, label := range got {
					// These captures omit the AfterStep hook type in Messages.
					// Require an honest adjacent-step label for that exact slot.
					expected := want[i]
					if strings.Contains(label, ":a hook between steps 1 and 2") {
						expected = strings.Replace(expected, ":a hook after step 1", ":a hook between steps 1 and 2", 1)
					}
					if label != expected {
						t.Fatalf("%s: hook labels JSON=%v Messages=%v", name, want, got)
					}
				}
			}
		}
	}
}

func TestRound14BackgroundAnnotationValidation(t *testing.T) {
	for _, a := range []Annotation{
		{Scenario: "s", Kind: KindKnown, Step: 1, BackgroundStep: 1, StepText: "x", Title: "bug"},
		{Scenario: "s", Kind: KindKnown, BackgroundStep: 1, Title: "bug"},
		{Scenario: "s", Kind: KindKnown, BackgroundStep: -1, StepText: "x", Title: "bug"},
	} {
		if _, err := normalize(renderOptions{Version: 2, Annotations: []Annotation{a}}); err == nil {
			t.Fatalf("accepted invalid annotation: %+v", a)
		}
	}
}
