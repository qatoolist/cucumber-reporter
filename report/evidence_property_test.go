package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pub "github.com/qatoolist/cucumber-reporter/cucumber"

	cucumber "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
	model "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"
)

// This mutation corpus is independent of the folding implementation. Every valid
// legacy fixture receives new failures at varied positions and execution levels.
func TestEvidenceFloorFixtureMutations(t *testing.T) {
	var files []string
	for _, root := range []string{"../testdata/runners", "../testdata/captured", "../testdata/plain-compat", "../testdata/round12"} {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() && filepath.Ext(path) == ".json" {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	checked := 0
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = cucumber.ParseEnhanced(bytes.NewReader(raw), cucumber.Limits{}); err != nil {
			continue
		}
		var original []map[string]any
		if json.Unmarshal(raw, &original) != nil || len(original) == 0 {
			continue
		}
		for kind := 0; kind < 6; kind++ {
			t.Run(fmt.Sprintf("%s/%d", file, kind), func(t *testing.T) {
				var features []map[string]any
				if err := json.Unmarshal(raw, &features); err != nil {
					t.Fatal(err)
				}
				rng := rand.New(rand.NewSource(int64(kind + len(raw))))
				feature := features[rng.Intn(len(features))]
				elements, _ := feature["elements"].([]any)
				marker := fmt.Sprintf("EVIDENCE_FLOOR_MUTATION_%d", kind)
				failed := func() map[string]any {
					return map[string]any{"name": marker, "result": map[string]any{"status": "failed", "error_message": marker}}
				}
				sc := map[string]any{"name": marker, "line": 999999, "steps": []any{failed()}}
				switch kind {
				case 0: // standalone scenario, including first and last positions
					at := rng.Intn(len(elements) + 1)
					elements = append(elements, nil)
					copy(elements[at+1:], elements[at:])
					elements[at] = sc
				case 1, 2: // external before/after hooks
					key := "before"
					if kind == 2 {
						key = "after"
					}
					sc["steps"] = []any{map[string]any{"result": map[string]any{"status": "passed"}}}
					sc[key] = []any{failed()}
					elements = append(elements, sc)
				case 3: // nested step hook
					sc["steps"] = []any{map[string]any{"result": map[string]any{"status": "passed"}, "after": []any{failed()}}}
					elements = append(elements, sc)
				case 4: // trailing Background must never disappear
					sc["type"] = "background"
					elements = append(elements, sc)
				case 5: // earlier Ruby attempt with final pending is not rescued
					sc["id"] = marker
					sc["steps"].([]any)[0].(map[string]any)["match"] = map[string]any{"location": "steps.rb:1"}
					elements = append(elements, sc, map[string]any{"id": marker, "name": marker, "line": 999999, "steps": []any{map[string]any{"match": map[string]any{"location": "steps.rb:1"}, "result": map[string]any{"status": "pending"}}}})
				}
				feature["elements"] = elements
				b, err := json.Marshal(features)
				if err != nil {
					t.Fatal(err)
				}
				for _, enhanced := range []bool{false, true} {
					f, err := cucumber.ParseEnhanced(bytes.NewReader(b), cucumber.Limits{})
					if err != nil {
						continue
					}
					opt := renderOptions{Version: 2}
					if enhanced {
						p, e := preparePublicRun([]runInput{{Name: "mutated", Features: f}}, opt, RunConfig{})
						if e != nil {
							continue
						}
						f, opt = p.Features, p.Options
					}
					var html []byte
					if enhanced {
						html, err = renderPublicModel(f, opt)
					} else {
						// Exercise the public parser and directory renderer, including publication.
						if _, err = pub.Parse(bytes.NewReader(b), pub.Limits{}); err == nil {
							dir := t.TempDir()
							if err = os.WriteFile(filepath.Join(dir, "cucumber.json"), b, 0o600); err != nil {
								t.Fatal(err)
							}
							var out string
							out, err = RenderDir(dir, Options{Version: 2})
							if err == nil {
								html, err = os.ReadFile(filepath.Join(out, "index.html"))
							}
						}
					}
					if err != nil {
						continue
					}
					if !bytes.Contains(html, []byte(marker)) {
						t.Fatalf("enhanced=%v lost inserted evidence", enhanced)
					}
					if bytes.Contains(html, []byte(`"label":"Passed`)) {
						t.Fatalf("enhanced=%v failed mutation became green", enhanced)
					}
				}
			})
		}
		checked++
	}
	if checked < 30 {
		t.Fatalf("only %d valid fixtures exercised", checked)
	}
}

func TestDisplayedEvidenceGuardDetectsDroppedResult(t *testing.T) {
	f := []cucumber.Feature{{Elements: []cucumber.Scenario{{Steps: []cucumber.Step{{Result: cucumber.Result{Status: cucumber.StatusFailed}}}}}}}
	if err := checkDisplayedEvidence(f, &pageData{}); err == nil || !strings.Contains(err.Error(), "the report was refused") {
		t.Fatalf("missing independent guard: %v", err)
	}
	// The model inventory must include failure evidence in earlier attempts too.
	previous := f[0].Elements[0]
	current := cucumber.Scenario{Steps: []cucumber.Step{{Result: cucumber.Result{Status: cucumber.StatusPassed}}}}
	model.ScenarioMetadata(&current).PreviousAttempts = []cucumber.Scenario{previous}
	f[0].Elements[0] = current
	if err := checkDisplayedEvidence(f, &pageData{}); err == nil {
		t.Fatal("lost retry evidence accepted")
	}
}

func TestEvidenceFloorMessagesMutations(t *testing.T) {
	var files []string
	for _, pattern := range []string{"../testdata/captured/*/*.ndjson", "../testdata/captured/*/*/*.ndjson", "../testdata/runners/*/*.ndjson"} {
		found, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, found...)
	}
	checked := 0
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = cucumber.ParseMessages(bytes.NewReader(raw), cucumber.Limits{}); err != nil {
			continue
		}
		for kind := 0; kind < 3; kind++ {
			t.Run(fmt.Sprintf("%s/%d", file, kind), func(t *testing.T) {
				lines := bytes.Split(raw, []byte("\n"))
				var candidates []int
				for i, line := range lines {
					if bytes.Contains(line, []byte(`"testStepFinished"`)) {
						candidates = append(candidates, i)
					}
				}
				if len(candidates) == 0 {
					return
				}
				index := candidates[rand.New(rand.NewSource(int64(len(raw)+kind))).Intn(len(candidates))]
				marker := "MESSAGES_EVIDENCE_FLOOR_MUTATION"
				var envelope map[string]any
				if err = json.Unmarshal(lines[index], &envelope); err != nil {
					t.Fatal(err)
				}
				result := envelope["testStepFinished"].(map[string]any)["testStepResult"].(map[string]any)
				result["status"] = "FAILED"
				result["message"] = marker
				lines[index], err = json.Marshal(envelope)
				if err != nil {
					t.Fatal(err)
				}
				if kind == 1 {
					lines[index] = bytes.Replace(lines[index], []byte(`"status":"FAILED"`), []byte(`"status":"FAILED","ſtatus":"PASSED"`), 1)
				}
				if kind == 2 {
					lines = append(lines, []byte(`{"testStepFinished":{"testCaseStartedId":"missing","testStepId":"missing","testStepResult":{"status":"FAILED","message":"MESSAGES_EVIDENCE_FLOOR_MUTATION"}}}`))
				}
				m, err := cucumber.ParseMessages(bytes.NewReader(bytes.Join(lines, []byte("\n"))), cucumber.Limits{})
				if err != nil {
					return
				}
				p, err := preparePublicRun([]runInput{{Name: "mutated", Attempts: m.Attempts}}, renderOptions{Version: 2}, RunConfig{})
				if err != nil {
					return
				}
				html, err := renderPublicModel(p.Features, p.Options)
				if err != nil {
					return
				}
				if !bytes.Contains(html, []byte(marker)) {
					t.Fatal("inserted failure disappeared")
				}
				// Green is allowed only for an explicitly rescued retry, with the failed
				// attempt retained. checkDisplayedEvidence independently checks that rule.
				if bytes.Contains(html, []byte(`"label":"Passed`)) {
					rescued := false
					for _, s := range p.Summary.Scenarios {
						for _, a := range p.Metadata[s.ID].Attempts {
							for _, st := range a.Steps {
								if st.Result.Error == marker && s.Status == "passed" {
									rescued = true
								}
							}
						}
					}
					if !rescued {
						t.Fatal("unrescued Messages failure became green")
					}
				}
			})
		}
		checked++
	}
	if checked < 17 {
		t.Fatalf("only %d streams exercised", checked)
	}
}
