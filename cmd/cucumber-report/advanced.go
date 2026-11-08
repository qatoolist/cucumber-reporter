package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/qatoolist/cucumber-reporter/cucumber"
	"github.com/qatoolist/cucumber-reporter/report"
)

// readRegular applies the same symlink/identity checks to every optional input.
func readRegular(name string, limit int64) ([]byte, error) {
	name = filepath.Clean(name)
	info, err := os.Lstat(name)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s: input is a symbolic link or not regular", name)
	}
	f, err := os.Open(name)
	if err != nil {
		return nil, errors.New("cannot open input")
	}
	defer func() { _ = f.Close() }()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, errors.New("input changed while opening")
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+4))
	if err != nil {
		return nil, errors.New("cannot read input")
	}
	b = bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("%s: %w (%d bytes)", name, cucumber.ErrTooLarge, limit)
	}
	return b, nil
}

func decodeFile(name string, v any, flag string) error {
	b, err := readRegular(name, report.HistoryByteLimit)
	if err != nil {
		return fmt.Errorf("%s: %w", flag, err)
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err = d.Decode(v); err != nil {
		if flag == "-run-config" {
			return fmt.Errorf("%s: invalid run configuration (check field names and value types)", flag)
		}
		return fmt.Errorf("%s: not a valid summary file; if it came from an earlier version, regenerate it", flag)
	}
	if _, err = d.Token(); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%s: trailing configuration data", flag)
	}
	return nil
}

type advancedFlags struct {
	config, baseline, history, format string
	validate, strict, bundle, fail    bool
}

func runAdvanced(paths []string, parent string, opt report.Options, flags advancedFlags, stdout, stderr io.Writer) int {
	cfg := report.RunConfig{}
	fail := func(err error) int { _, _ = fmt.Fprintf(stderr, "cucumber-report: %v\n", err); return 1 }
	if flags.config != "" {
		if err := decodeFile(flags.config, &cfg, "-run-config"); err != nil {
			return fail(err)
		}
	}
	if flags.baseline != "" {
		var s report.RunSummary
		if err := decodeFile(flags.baseline, &s, "-baseline"); err != nil {
			return fail(err)
		}
		if s.Version > report.SummaryVersion {
			return fail(fmt.Errorf("-baseline: summary schema %d was written by a newer version; upgrade this tool", s.Version))
		}
		if s.Version != report.SummaryVersion {
			return fail(fmt.Errorf("-baseline: summary schema %d is unsupported; regenerate it with this version (schema %d)", s.Version, report.SummaryVersion))
		}
		cfg.Baseline = &s
	}
	if flags.history != "" {
		if err := decodeFile(flags.history, &cfg.History, "-history"); err != nil {
			return fail(err)
		}
		for _, s := range cfg.History {
			if s.Version > report.SummaryVersion {
				return fail(fmt.Errorf("-history: summary schema %d was written by a newer version; upgrade this tool", s.Version))
			}
			if s.Version != report.SummaryVersion {
				return fail(fmt.Errorf("-history: summary schema %d is unsupported; regenerate it with this version (schema %d)", s.Version, report.SummaryVersion))
			}
		}
	}
	cfg.Bundle = cfg.Bundle || flags.bundle
	cfg.Gates.Unexpected = cfg.Gates.Unexpected || flags.fail
	flags.format = strings.ToLower(flags.format)
	if !strings.EqualFold(flags.format, "json") && !strings.EqualFold(flags.format, "messages") {
		return fail(errors.New("format must be json or messages"))
	}
	var inputs []report.Input
	var diagnostics []report.Diagnostic
	var measuredRuns []cucumber.MessageRun

	nameCounts := map[string]int{}
	for _, name := range paths {
		nameCounts[filepath.Base(filepath.Clean(name))]++
	}
	seenInputPaths := map[string]string{}
	for i, name := range paths {
		absolute, _ := filepath.Abs(filepath.Clean(name))
		if previous, ok := seenInputPaths[absolute]; ok {
			repeated := fmt.Sprintf("input %q given twice", previous)
			if current := shortestInputName(name, paths); current != previous {
				repeated = fmt.Sprintf("input %q given twice (also as %q)", previous, current)
			}
			return fail(fmt.Errorf("%w: %s; merging reruns or matrix cells across inputs is not supported in v0.3.0; use one report per run or --retry within a single run", report.ErrInvalidInput, repeated))
		}
		seenInputPaths[absolute] = shortestInputName(name, paths)
		inputPath := name
		info, err := os.Lstat(filepath.Clean(name))
		if err != nil {
			return fail(fmt.Errorf("cannot inspect input %s: %w", filepath.Clean(name), err))
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fail(report.ErrNotRegular)
		}
		if info.IsDir() {
			inputPath = filepath.Join(name, "cucumber.json")
			if flags.format == "messages" {
				inputPath = filepath.Join(name, "cucumber.ndjson")
			}
		}
		b, err := readRegular(inputPath, opt.Limits.WithDefaults().Input)
		if err != nil && info.IsDir() && errors.Is(err, os.ErrNotExist) {
			return fail(fmt.Errorf("%s: no results file; for nested shard directories, list each shard as a separate input", filepath.Clean(inputPath)))
		}
		if err != nil {
			return fail(err)
		}
		if flags.format == "json" && (strings.HasSuffix(strings.ToLower(inputPath), ".ndjson") || bytes.HasPrefix(bytes.TrimSpace(b), []byte(`{"meta"`))) {
			return fail(fmt.Errorf("%s: looks like Cucumber Messages; use -format messages", filepath.Clean(inputPath)))
		}
		var features []cucumber.Feature
		var attempts []cucumber.MessageAttempt
		var inputDiagnostics []report.Diagnostic
		if flags.format == "messages" {
			var mr cucumber.MessageRun
			mr, err = cucumber.ParseMessages(bytes.NewReader(b), opt.Limits)
			if err != nil {
				return fail(fmt.Errorf("%s: %w", filepath.Clean(inputPath), err))
			}
			attempts = mr.Attempts
			measuredRuns = append(measuredRuns, mr)
		} else {
			features, err = cucumber.Parse(bytes.NewReader(b), opt.Limits)
			if err != nil {
				return fail(fmt.Errorf("%s: %w", filepath.Clean(inputPath), err))
			}
			ds, err := report.ValidateInput(b, opt.Limits)
			if err != nil {
				return fail(err)
			}
			for _, d := range ds {
				d.Path = fmt.Sprintf("input[%d].%s", i, d.Path)
				inputDiagnostics = append(inputDiagnostics, d)
			}
		}
		logicalName := filepath.Base(filepath.Clean(name))
		if nameCounts[logicalName] > 1 {
			logicalName = shortestInputName(name, paths)
		}
		for j, other := range inputs {
			if other.Name == logicalName {
				return fail(fmt.Errorf("%w: inputs %q and %q both resolve to the name %q; give them distinguishing directories", report.ErrInvalidInput, filepath.Clean(paths[j]), filepath.Clean(name), logicalName))
			}
		}
		diagnostics = append(diagnostics, inputDiagnostics...)
		inputs = append(inputs, report.Input{Name: logicalName, Features: features, Attempts: attempts, Diagnostics: inputDiagnostics})
	}
	applyMessageFacts(measuredRuns, &opt, &cfg)
	if flags.strict && len(diagnostics) > 0 {
		for _, d := range diagnostics {
			_, _ = fmt.Fprintf(stderr, "%s: %s\n", d.Path, d.Message)
		}
		return fail(errStrictDiagnostics)
	}

	prepared, err := report.PrepareRun(inputs, opt, cfg)
	if err != nil {
		if flags.baseline != "" && strings.HasPrefix(err.Error(), "baseline") {
			err = fmt.Errorf("-baseline: %w", err)
		}
		if flags.history != "" && strings.HasPrefix(err.Error(), "history") {
			err = fmt.Errorf("-history: %w", err)
		}
		return fail(err)
	}
	for _, d := range prepared.Summary.Diagnostics {
		fmt.Fprintf(stderr, "%s: %s\n", d.Path, d.Message)
	}
	if flags.strict {
		for _, d := range prepared.Summary.Diagnostics {
			if d.Path != "runner.outcome" {
				return fail(errStrictDiagnostics)
			}
		}
	}
	if flags.validate {
		_, _ = fmt.Fprintf(stdout, "Valid: %d scenarios; %s\n", prepared.Summary.ObservedScenarios, diagnosticCount(len(prepared.Summary.Diagnostics)))
		if len(prepared.Summary.GateFailures) > 0 {
			for _, g := range prepared.Summary.GateFailures {
				_, _ = fmt.Fprintf(stderr, "Quality gate: %s\n", g)
			}
			return 3
		}
		return 0
	}
	if parent == "" {
		parent = paths[0]
		if info, _ := os.Lstat(parent); info != nil && !info.IsDir() {
			parent = filepath.Dir(parent)
		}
	}
	dir, err := report.WriteRunDir(prepared, parent)
	if err != nil && !errors.Is(err, report.ErrGateFailed) {
		return fail(err)
	}
	warnOutputSize(dir, stderr)
	_, _ = fmt.Fprintf(stdout, "HTML report: %s\n", filepath.Join(dir, "index.html"))
	if errors.Is(err, report.ErrGateFailed) {
		for _, g := range prepared.Summary.GateFailures {
			_, _ = fmt.Fprintf(stderr, "Quality gate: %s\n", g)
		}
		return 3
	}
	return 0
}

// Explicit configuration takes precedence over runner-reported facts.
func applyMessageFacts(runs []cucumber.MessageRun, opt *report.Options, cfg *report.RunConfig) {
	if len(runs) == 0 {
		return
	}
	branch, revision := runs[0].Branch, runs[0].Revision
	var earliest, latest time.Time
	haveAllTimes := true
	for i, run := range runs {
		if branch != run.Branch {
			branch = ""
		}
		if revision != run.Revision {
			revision = ""
		}
		started, e1 := time.Parse(time.RFC3339Nano, run.Started)
		finished, e2 := time.Parse(time.RFC3339Nano, run.Finished)
		if e1 == nil && e2 == nil && !finished.Before(started) {
			cfg.Phases = append(cfg.Phases, report.Phase{Name: fmt.Sprintf("Runner stream %d", i+1), Started: run.Started, Finished: run.Finished})
			if earliest.IsZero() || started.Before(earliest) {
				earliest = started
			}
			if latest.IsZero() || finished.After(latest) {
				latest = finished
			}
		} else {
			haveAllTimes = false
		}
		if run.Remote != "" {
			opt.RunFacts = append(opt.RunFacts, report.Fact{Label: fmt.Sprintf("Stream %d repository", i+1), Value: run.Remote})
		}
	}
	if branch != "" {
		opt.RunFacts = append(opt.RunFacts, report.Fact{Label: "Runner branch", Value: branch})
	}
	if revision != "" {
		opt.RunFacts = append(opt.RunFacts, report.Fact{Label: "Runner revision", Value: revision})
	}
	if opt.WallClock == "" && haveAllTimes && !earliest.IsZero() {
		opt.WallClock = latest.Sub(earliest).String()
	}
}

// errStrictDiagnostics ends every -validate -strict refusal caused by diagnostics.
var errStrictDiagnostics = errors.New("strict validation rejected run diagnostics")

// Preserve enough suffix components to identify the input without publishing an
// absolute CI workstation path. A name that is a suffix of another input keeps its
// cleaned path, without a leading separator. Identical arguments fall back to the
// base name of an absolute argument, or to the cleaned relative argument.
func shortestInputName(name string, names []string) string {
	cleaned := filepath.Clean(name)
	parts := strings.Split(strings.Trim(filepath.ToSlash(cleaned[len(filepath.VolumeName(cleaned)):]), "/"), "/")
	for count := 1; count <= len(parts); count++ {
		suffix := strings.Join(parts[len(parts)-count:], "/")
		matches := 0
		for _, other := range names {
			value := strings.Trim(filepath.ToSlash(filepath.Clean(other)), "/")
			if value == suffix || strings.HasSuffix(value, "/"+suffix) {
				matches++
			}
		}
		if matches == 1 {
			return suffix
		}
	}
	full := strings.Join(parts, "/")
	for _, other := range names {
		if value := strings.Trim(filepath.ToSlash(filepath.Clean(other)), "/"); value != full && strings.HasSuffix(value, "/"+full) {
			// A suffix of a different input keeps its distinguishing components.
			return full
		}
	}
	if filepath.IsAbs(cleaned) {
		return filepath.Base(cleaned)
	}
	return full
}
