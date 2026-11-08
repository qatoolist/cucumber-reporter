package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/qatoolist/cucumber-reporter/cucumber"
	input "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
	"github.com/qatoolist/cucumber-reporter/report"
)

func validatePlain(path string, opt report.Options, strict bool, stdout, stderr io.Writer) int {
	fail := func(err error) int { fmt.Fprintf(stderr, "cucumber-report: %v\n", err); return 1 }
	info, err := os.Lstat(filepath.Clean(path))
	if err != nil {
		return fail(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fail(report.ErrNotRegular)
	}
	if info.IsDir() {
		path = filepath.Join(path, "cucumber.json")
	}
	raw, err := readRegular(path, opt.Limits.WithDefaults().Input)
	if err != nil {
		return fail(err)
	}
	if input.LooksLikeMessages(raw) {
		return fail(fmt.Errorf("messages NDJSON input requires -validate -format messages"))
	}
	features, err := cucumber.Parse(bytes.NewReader(raw), opt.Limits)
	if err != nil {
		return fail(err)
	}
	if err = report.Validate(features, opt); err != nil {
		return fail(err)
	}
	diagnostics, err := report.ValidateInput(raw, opt.Limits)
	diagnostics = append(diagnostics, annotationTextDiagnostics(features, opt)...)
	if err != nil {
		return fail(err)
	}
	for _, d := range diagnostics {
		fmt.Fprintf(stderr, "%s: %s\n", d.Path, d.Message)
	}
	if strict && len(diagnostics) > 0 {
		return fail(errStrictDiagnostics)
	}
	folded, err := input.Parse(bytes.NewReader(raw), input.Limits(opt.Limits))
	if err != nil {
		return fail(err)
	}
	n := 0
	for _, f := range folded {
		for _, sc := range f.Elements {
			if sc.Type != "background" {
				n++
			}
		}
	}
	fmt.Fprintf(stdout, "Valid plain report: %d scenarios; outcome %s; %s\n", n, plainOutcome(folded), diagnosticCount(len(diagnostics)))
	return 0
}

func plainOutcome(f []input.Feature) input.Status {
	count := 0
	for _, feature := range f {
		count += len(feature.Elements)
	}
	if count == 0 {
		return input.StatusUndefined
	}
	return input.RunStatus(f)
}

func diagnosticCount(n int) string {
	word := "diagnostics"
	if n == 1 {
		word = "diagnostic"
	}
	return fmt.Sprintf("%d %s", n, word)
}

func annotationTextDiagnostics(features []cucumber.Feature, opt report.Options) []report.Diagnostic {
	ids := report.ScenarioIDs(features)
	var diagnostics []report.Diagnostic
	for i, a := range opt.Annotations {
		if a.Step == 0 || a.StepText == "" {
			continue
		}
		for fi, f := range features {
			for si, sc := range f.Elements {
				if ids[fi][si] != a.Scenario {
					continue
				}
				own := 0
				for _, step := range sc.Steps {
					if step.Hidden || (step.Provenance != nil && step.Provenance.Origin != "" && step.Provenance.Origin != "own") {
						continue
					}
					own++
					if own == a.Step && step.Name != a.StepText {
						diagnostics = append(diagnostics, report.Diagnostic{Path: fmt.Sprintf("annotations[%d]", i), Message: "Annotation stepText does not match its declared step; it covers no failure"})
					}
				}
			}
		}
	}
	return diagnostics
}
