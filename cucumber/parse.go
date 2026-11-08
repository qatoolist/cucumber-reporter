package cucumber

import (
	"errors"
	"io"
	"strconv"

	input "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
	modelmeta "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"
)

// ParseError identifies a safe numeric location or byte offset in invalid input.
type ParseError struct {
	// Path contains safe array indices and fixed field names.
	Path string
	// Offset is a byte offset, or -1 when unavailable.
	Offset int64
	// Err is the underlying input error.
	Err error
}

// Error describes the safe input location.
func (e *ParseError) Error() string {
	if e.Path != "" {
		return e.Err.Error() + " at " + e.Path
	}
	if e.Offset >= 0 {
		return e.Err.Error() + " at byte " + strconv.FormatInt(e.Offset, 10)
	}
	return e.Err.Error()
}

// Unwrap exposes the underlying error for errors.Is and errors.As.
func (e *ParseError) Unwrap() error { return e.Err }

func publicError(err error) error {
	var p *input.ParseError
	if errors.As(err, &p) {
		return &ParseError{Path: p.Path, Offset: p.Offset, Err: p.Err}
	}
	return err
}

// Parse reads legacy Cucumber JSON, preserving scenario steps, hook evidence and
// Background elements. Unknown non-execution fields are ignored. Empty scenarios
// remain undefined. Report rendering folds Backgrounds and accounts for hooks.
// Valid empty arrays and features without scenarios are accepted. Limits.WithDefaults
// supplies omitted limits; syntax and validation failures support errors.Is and
// errors.As with *ParseError. The returned values retain explicit provenance so
// copies preserve execution evidence and diagnostics.
func Parse(r io.Reader, lim Limits) ([]Feature, error) {
	f, e := input.ParseEnhanced(r, modelmeta.Limits(lim))
	return fromInternal(f), publicError(e)
}

// ParseStream has the same contract as Parse.
//
// Deprecated: use Parse.
func ParseStream(r io.Reader, lim Limits) ([]Feature, error) { return Parse(r, lim) }

// ParseEnhanced has the same contract as Parse, including valid empty arrays.
//
// Deprecated: use Parse.
func ParseEnhanced(r io.Reader, lim Limits) ([]Feature, error) {
	f, e := input.ParseEnhanced(r, modelmeta.Limits(lim))
	return fromInternal(f), publicError(e)
}

// Validate checks statuses, completeness and nonnegative durations without rendering.
// Nil input and empty features are valid empty executions. A step without a result
// status is incomplete; a scenario without steps remains undefined.
func Validate(features []Feature) error {
	return publicError(input.Validate(internalFeatures(features)))
}
