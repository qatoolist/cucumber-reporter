package report

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	cucumber "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

// These fixtures are formatter-shaped examples, not captured runner output.
// Expected results and intentionally refused ambiguous shapes live in truth.json.
func TestRunnerFormatCorpus(t *testing.T) {
	files, err := filepath.Glob("../testdata/runners/*/truth.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		name := filepath.Base(filepath.Dir(file))
		t.Run(name, func(t *testing.T) {
			var truth struct {
				Counts        map[string]int `json:"counts"`
				EnhancedError bool           `json:"enhancedError"`
			}
			b, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(b, &truth); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Dir(file)
			raw, err := os.ReadFile(filepath.Join(dir, "cucumber.json"))
			messages := os.IsNotExist(err)
			if messages {
				raw, err = os.ReadFile(filepath.Join(dir, "cucumber.ndjson"))
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, enhanced := range []bool{false, true} {
				if messages && !enhanced {
					continue
				}
				in := runInput{Name: "fixture"}
				var parseErr error
				if messages {
					m, e := cucumber.ParseMessages(bytes.NewReader(raw), cucumber.Limits{})
					parseErr = e
					in.Attempts = m.Attempts
				} else {
					parse := cucumber.Parse
					if enhanced {
						parse = cucumber.ParseEnhanced
					}
					in.Features, parseErr = parse(bytes.NewReader(raw), cucumber.Limits{})
				}
				refused := strings.HasPrefix(name, "behave-") || strings.HasSuffix(name, "-truncated") || strings.HasSuffix(name, "-crashed")
				if refused {
					if parseErr == nil {
						t.Fatal("incomplete or unsupported fixture accepted")
					}
					continue
				}
				if parseErr != nil {
					t.Fatalf("enhanced=%v: %v", enhanced, parseErr)
				}
				opt, _ := normalize(renderOptions{Version: 2})
				if enhanced {
					p, e := preparePublicRun([]runInput{in}, opt, RunConfig{})
					if truth.EnhancedError {
						if e == nil {
							t.Fatal("ambiguous duplicate accepted")
						}
						continue
					}
					if e != nil {
						t.Fatal(e)
					}
					in.Features = p.Features
					opt = p.Options
				}
				data, e := buildPage(in.Features, opt, nil)
				if e != nil {
					t.Fatal(e)
				}
				counts := map[string]int{}
				for _, f := range data.Features {
					for _, s := range f.Elements {
						if !s.RunHook {
							counts[s.Status]++
						}
					}
				}
				if !reflect.DeepEqual(counts, truth.Counts) {
					t.Fatalf("enhanced=%v: got %v; want %v", enhanced, counts, truth.Counts)
				}
			}
		})
	}
}
