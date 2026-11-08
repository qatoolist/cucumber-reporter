// Package evidence independently inventories execution results before normalization.
// It deliberately knows nothing about scenario folding, identities or rendering.
package evidence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"
)

// FloorError is a contradiction found by the evidence floor. Callers classify it
// by type (errors.As), never by message text, so input text cannot forge it.
type FloorError struct{ err error }

func (e *FloorError) Error() string { return e.err.Error() }

// Unwrap returns the underlying formatted error.
func (e *FloorError) Unwrap() error { return e.err }

// Floorf creates a FloorError; the format follows fmt.Errorf.
func Floorf(format string, args ...any) error {
	return &FloorError{fmt.Errorf(format, args...)}
}

// Counts is a multiset of non-passing raw results, plus the runner's failure signal.
type Counts struct {
	// Scenarios inventories legacy elements in source order, independently of the model.
	Scenarios    []map[string]int
	Results      map[string]int
	RunnerFailed bool
}

// Fold follows Unicode simple folding, including long-s and Kelvin-sign aliases.
func Fold(s string) string {
	return strings.Map(func(r rune) rune {
		m := r
		for next := unicode.SimpleFold(r); next != r; next = unicode.SimpleFold(next) {
			if next < m {
				m = next
			}
		}
		return m
	}, s)
}

// Scan walks JSON tokens without maps that could overwrite duplicate result keys.
// Multiple status values on one result take the worst outcome, never the last.
func Scan(raw []byte) (Counts, error) {
	out := Counts{Results: map[string]int{}}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	scope := -1
	var walk func(string, int) (any, error)
	walk = func(parent string, depth int) (any, error) {
		if depth > 1024 {
			return nil, fmt.Errorf("evidence: nesting exceeds limit")
		}
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		delim, ok := tok.(json.Delim)
		if !ok {
			return tok, nil
		}
		if delim == '[' {
			for dec.More() {
				previous := scope
				if parent == Fold("elements") && depth == 2 {
					scope = len(out.Scenarios)
					out.Scenarios = append(out.Scenarios, nil)
				}
				if _, err = walk("", depth+1); err != nil {
					return nil, err
				}
				scope = previous
			}
			_, err = dec.Token()
			return nil, err
		}
		if delim != '{' {
			return nil, fmt.Errorf("evidence: invalid container")
		}
		status := ""
		hasStatus := false
		hasError := false
		for dec.More() {
			key, e := dec.Token()
			if e != nil {
				return nil, e
			}
			k := Fold(key.(string))
			v, e := walk(k, depth+1)
			if e != nil {
				return nil, e
			}
			if k == Fold("status") {
				hasStatus = true
				s, ok := v.(string)
				s = strings.ToLower(s)
				if s == "unknown" {
					s = "pending"
				}
				if !ok || (s != "" && rank(s) == 0) {
					s = "undefined"
				}
				if rank(s) > rank(status) {
					status = s
				}
			}
			if k == Fold("error_message") {
				if s, ok := v.(string); ok && s != "" {
					hasError = true
				}
			}
			if parent == Fold("testRunFinished") && k == Fold("success") && v == false {
				out.RunnerFailed = true
			}
		}
		_, err = dec.Token()
		if err != nil {
			return nil, err
		}
		if parent == Fold("result") || parent == Fold("testStepResult") {
			if (!hasStatus || status == "") && hasError {
				status = "failed"
			}
			if status != "" && status != "passed" {
				out.Results[status]++
				if scope >= 0 {
					if out.Scenarios[scope] == nil {
						out.Scenarios[scope] = map[string]int{}
					}
					out.Scenarios[scope][status]++
				}
			}
		}
		return nil, nil
	}
	if _, err := walk("", 0); err != nil {
		return out, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return out, fmt.Errorf("evidence: trailing data")
	}
	return out, nil
}

func rank(s string) int {
	switch s {
	case "failed":
		return 6
	case "ambiguous":
		return 5
	case "undefined", "unknown":
		return 4
	case "pending":
		return 3
	case "skipped":
		return 2
	case "passed":
		return 1
	}
	return 0
}

// Check rejects disappeared non-passing results. A worse represented result may
// account for an input result normalized from malformed runner metadata.
func Check(raw, represented Counts) error {
	available := map[string]int{}
	for k, v := range represented.Results {
		available[k] = v
	}
	missing := 0
	for _, status := range []string{"failed", "ambiguous", "undefined", "unknown", "pending", "skipped"} {
		need := raw.Results[status]
		for _, candidate := range []string{status, "failed", "ambiguous", "undefined", "pending", "skipped"} {
			if rank(candidate) < rank(status) {
				continue
			}
			n := min(need, available[candidate])
			available[candidate] -= n
			need -= n
		}
		missing += need
	}
	if missing > 0 {
		return Floorf("evidence floor: %d non-passing results in the input are not represented; internal consistency check failed, so the report was refused (please report this with the input)", missing)
	}
	return nil
}

// Add records one normalized result without sharing a parser's status machinery.
func (c *Counts) Add(status string) {
	status = strings.ToLower(status)
	if status == "unknown" {
		status = "pending"
	}
	if status != "" && status != "passed" {
		if c.Results == nil {
			c.Results = map[string]int{}
		}
		c.Results[status]++
	}
}
