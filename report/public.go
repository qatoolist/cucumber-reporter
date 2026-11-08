package report

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"unicode/utf8"

	input "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"

	"github.com/qatoolist/cucumber-reporter/cucumber"
	modelmeta "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"
	"github.com/qatoolist/cucumber-reporter/internal/evidence"
)

// FeatureIDs returns each feature navigation ID, indexed like f. IDs use the
// feature URI basename slug, with -2, -3 and subsequent suffixes on collisions.
// Synthetic run hooks use a reserved slug after ordinary feature IDs are allocated.
func FeatureIDs(f []cucumber.Feature) []string { return featureIDs(internalFeatures(f)) }

// ScenarioIDs returns v0.2.0-compatible navigation IDs, indexed like f[i].Elements[j].
// Each ID uses its feature ID and source line, with suffixes for collisions in
// source order. Missing source lines use zero. Folded source slots
// retain their IDs for bookmarks and may redirect to their surviving scenario.
func ScenarioIDs(f []cucumber.Feature) [][]string { return scenarioIDs(internalFeatures(f)) }

// RunScenarioIDs returns enhanced navigation IDs independent of shard order.
func RunScenarioIDs(f []cucumber.Feature) [][]string { return runScenarioIDs(internalFeatures(f)) }

// ScenarioOutcomes classifies scenarios using the supplied issue annotations,
// indexed like f[i].Elements[j]. Outcomes match the rendered scenario: known for
// fully covered known failures, otherwise an effective execution status. Source
// Background and retry slots folded into another scenario return OutcomeFolded;
// exclude those slots when counting. Invalid annotations return ErrInvalidOptions.
func ScenarioOutcomes(f []cucumber.Feature, a []Annotation) ([][]string, error) {
	return publicResult(scenarioOutcomes(internalFeatures(f), a))
}

// Render creates a deterministic offline HTML report. Backgrounds fold into their
// following scenario; non-passing hooks contribute their evidence and outcome.
// Nil input renders an informational empty execution. Options returned by PrepareRun
// must accompany the matching prepared Features; a mismatch returns ErrInvalidOptions.
func Render(f []cucumber.Feature, opt Options) ([]byte, error) {
	if err := checkPreparedFeatures(f, opt); err != nil {
		return nil, err
	}
	return publicResult(renderPublicModel(renderFeatures(f, internalOptions(opt)), internalOptions(opt)))
}

// RenderTo streams an offline HTML report to w, with the same model/options contract
// as Render. A write error may leave a partial document; publish only on success.
func RenderTo(w io.Writer, f []cucumber.Feature, opt Options) error {
	if err := checkPreparedFeatures(f, opt); err != nil {
		return err
	}
	return publicReportError(renderTo(w, renderFeatures(f, internalOptions(opt)), internalOptions(opt)))
}

// RenderFeaturesDirTo publishes an already parsed run into a new html-* directory
// beneath an existing parent. It returns that directory after atomic publication.
// Prepared options must accompany their matching prepared Features. A direct
// parent symlink is refused; symlinks in parent path components are followed.
// Output is written with owner-only permissions and earlier reports are never
// replaced. A failed render leaves no new published directory.
func RenderFeaturesDirTo(f []cucumber.Feature, parent string, opt Options) (string, error) {
	if err := checkPreparedFeatures(f, opt); err != nil {
		return "", err
	}
	return publicResult(renderFeaturesDirTo(renderFeatures(f, internalOptions(opt)), parent, internalOptions(opt)))
}

// Validate checks the same model and annotations as plain rendering, without files.
func Validate(f []cucumber.Feature, opt Options) error {
	if err := checkPreparedFeatures(f, opt); err != nil {
		return err
	}
	return publicReportError(validateModel(internalFeatures(f), internalOptions(opt)))
}

// Input is one shard with a required, unique Name. Attempts preserves Messages executions; use either
// Features or Attempts in an input, never both. Diagnostics carries the caller's notices about this
// input, typically ValidateInput results with their Path prefixed by the input position; PrepareRun
// appends them, in input order, to the prepared Summary.Diagnostics so they are published and covered
// by the prepared digest. Their text is cleaned of control and bidi characters and bounded (2 KiB
// per message, 200 diagnostics plus one omission notice). The reserved path runner.outcome and
// absolute filesystem paths return ErrInvalidOptions.
type Input struct {
	Name        string
	Features    []cucumber.Feature
	Attempts    []cucumber.MessageAttempt
	Diagnostics []Diagnostic
}

// PreparedRun is a copied render/export snapshot obtained from PrepareRun. Its
// fields are explicit public values; copies preserve rendering behavior. Treat the
// snapshot as immutable after preparation. A Go value copy is shallow: maps,
// slices and Options.Analysis remain shared; prepare again to obtain independent state. Changed feature values must be prepared
// again; render and publication reject a mismatch with the prepared digest.
type PreparedRun struct {
	Features    []cucumber.Feature
	Options     Options
	Summary     RunSummary
	History     []RunSummary
	Metadata    map[string]ScenarioMeta
	Bundle      bool
	OmitHistory bool
}

// PrepareRun merges disjoint shards, applies redaction and computes optional policies.
// Repeated scenario locations across inputs return ErrInvalidInput. Retries within
// one supported runner input remain supported. Inputs need unique nonempty Names.
// It copies model data before applying redaction and does not modify the caller
// model. Nil baseline and history disable their optional analysis.
// Parsed features carry a raw case audit taken when they were parsed; editing parsed
// results to a greener outcome before PrepareRun returns ErrEvidence naming the edited
// scenario. Parse the edited input again instead.
func PrepareRun(inputs []Input, opt Options, cfg RunConfig) (*PreparedRun, error) {
	callerDiagnostics, err := inputDiagnostics(inputs)
	if err != nil {
		return nil, err
	}
	in := make([]runInput, len(inputs))
	for i, v := range inputs {
		in[i] = runInput{Name: v.Name, Features: internalFeatures(v.Features), Attempts: internalAttempts(v.Attempts), Diagnostics: callerDiagnostics[i]}
	}
	p, e := preparePublicRun(in, internalOptions(opt), cfg)
	if e != nil {
		return nil, publicReportError(e)
	}

	out := &PreparedRun{Features: publicFeatures(p.Features), Options: publicOptions(p.Options), Summary: p.Summary, History: p.History, Metadata: p.Metadata, Bundle: p.Bundle, OmitHistory: p.OmitHistory}
	out.Options.Analysis = &out.Summary
	out.Options.Execution = p.Metadata
	out.Options.PreparedDigest = preparedDigest(out.Features, out.Options.Analysis, out.Options.Execution)
	return out, nil
}

// Caller diagnostics are published, so they are cleaned and bounded like other
// report text, and cannot claim a path the reporter reserves for its own findings.
const (
	maxInputDiagnostics       = 200
	maxInputDiagnosticMessage = 2048
)

// inputDiagnostics validates caller diagnostics per input. The run-wide cap keeps
// input order; the omission notice travels with the last input. Preparation
// publishes them through the same redaction as every other summary value.
func inputDiagnostics(inputs []Input) ([][]Diagnostic, error) {
	out := make([][]Diagnostic, len(inputs))
	kept, omitted := 0, 0
	for i, v := range inputs {
		for j, d := range v.Diagnostics {
			path := diagnosticText(d.Path)
			if strings.EqualFold(strings.TrimSpace(path), "runner.outcome") {
				return nil, invalid("inputs[%d].diagnostics[%d].path runner.outcome is reserved for the reporter", i, j)
			}
			if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\") || (len(path) > 2 && path[1] == ':' && (path[2] == '\\' || path[2] == '/')) {
				return nil, invalid("inputs[%d].diagnostics[%d].path must be a structural location, not a filesystem path", i, j)
			}
			if kept == maxInputDiagnostics {
				omitted++
				continue
			}
			kept++
			out[i] = append(out[i], Diagnostic{Path: path, Message: diagnosticText(d.Message)})
		}
	}
	if omitted > 0 {
		last := len(out) - 1
		out[last] = append(out[last], Diagnostic{Path: "input.diagnostics", Message: fmt.Sprintf("%d more input diagnostics omitted", omitted)})
	}
	return out, nil
}

func diagnosticText(s string) string {
	s = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ", "\t", " ").Replace(clean(s))
	if len(s) <= maxInputDiagnosticMessage {
		return s
	}
	cut := maxInputDiagnosticMessage
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…"
}

// WriteRunDir publishes a prepared snapshot and its exports atomically.
// When quality gates fail it returns both the published directory and ErrGateFailed.
func WriteRunDir(p *PreparedRun, parent string) (string, error) {
	if p == nil {
		return "", invalid("prepared run is required")
	}
	if p.Options.PreparedDigest != preparedDigest(p.Features, &p.Summary, p.Metadata) {
		return "", invalid("prepared summary or metadata changed; prepare the modified run again")
	}
	if err := checkPreparedFeatures(p.Features, p.Options); err != nil {
		return "", err
	}
	return publicResult(writeRunDir(&preparedRun{Features: renderFeatures(p.Features, internalOptions(p.Options)), Options: internalOptions(p.Options), Summary: p.Summary, History: p.History, Metadata: p.Metadata, Bundle: p.Bundle, OmitHistory: p.OmitHistory}, parent))
}

// ValidateInput reports unsupported fields using safe schema paths.
func ValidateInput(b []byte, lim cucumber.Limits) ([]Diagnostic, error) {
	return publicResult(validateInput(b, modelmeta.Limits(lim)))
}

// RenderContext renders an offline report and checks cancellation while building
// the model and writing the template. No output is returned on cancellation.
func RenderContext(ctx context.Context, f []cucumber.Feature, opt Options) ([]byte, error) {
	if err := checkPreparedFeatures(f, opt); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	internal := internalOptions(opt)
	internal.renderContext = ctx
	return publicResult(renderPublicModel(renderFeatures(f, internal), internal))
}

// RenderToContext streams a report; on cancellation w may contain a partial
// document, so callers should publish only after this method succeeds.
func RenderToContext(ctx context.Context, w io.Writer, f []cucumber.Feature, opt Options) error {
	if err := checkPreparedFeatures(f, opt); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	internal := internalOptions(opt)
	internal.renderContext = ctx
	return publicReportError(renderTo(w, renderFeatures(f, internal), internal))
}

// RenderDir reads cucumber.json and publishes a report in the run directory.
// The run directory must not be a symbolic link; cucumber.json must be a regular
// non-symlink file. See RenderFeaturesDirTo for publication rules.
// Prepared options are refused: use RenderFeaturesDirTo with the prepared features.
func RenderDir(dir string, opt Options) (string, error) {
	if err := checkDirectoryOptions(opt); err != nil {
		return "", err
	}
	return publicResult(renderDir(dir, internalOptions(opt)))
}

// RenderDirTo reads cucumber.json and publishes into an existing output directory.
// It applies the input restrictions of RenderDir and publication rules of
// RenderFeaturesDirTo.
// Prepared options are refused: use RenderFeaturesDirTo with the prepared features.
func RenderDirTo(dir, parent string, opt Options) (string, error) {
	if err := checkDirectoryOptions(opt); err != nil {
		return "", err
	}
	return publicResult(renderDirTo(dir, parent, internalOptions(opt)))
}

// Equal compares every value in the public snapshot.
// PreparedRun values should be obtained through PrepareRun rather than constructed.
func (p PreparedRun) Equal(other PreparedRun) bool {
	return reflect.DeepEqual(p, other)
}

func publicReportError(err error) error {
	var floor *evidence.FloorError
	if errors.As(err, &floor) && !errors.Is(err, ErrEvidence) {
		return fmt.Errorf("%w: %w", ErrEvidence, err)
	}
	var pe *input.ParseError
	if errors.As(err, &pe) {
		return &cucumber.ParseError{Path: pe.Path, Offset: pe.Offset, Err: pe.Err}
	}
	return err
}
func publicResult[T any](value T, err error) (T, error) { return value, publicReportError(err) }

func preparedDigest(f []cucumber.Feature, summary *RunSummary, metadata map[string]ScenarioMeta) string {
	b, err := json.Marshal(struct {
		Features []cucumber.Feature
		Summary  *RunSummary
		Metadata map[string]ScenarioMeta
	}{f, summary, metadata})
	if err != nil {
		return "invalid"
	}
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

func checkPreparedFeatures(f []cucumber.Feature, opt Options) error {
	if opt.PreparedDigest == "" && (opt.Analysis != nil || opt.Execution != nil) {
		return invalid("analysis and execution require a matching prepared digest; use PrepareRun")
	}
	if opt.PreparedDigest != "" && opt.PreparedDigest != preparedDigest(f, opt.Analysis, opt.Execution) {
		return invalid("prepared run was edited after PrepareRun: its features, Summary or Metadata no longer match the prepared digest; call PrepareRun again on the edited input")
	}
	return nil
}

// A directory API reparses a source artifact, not the prepared snapshot. Its raw
// feature values cannot safely be associated with another run's analysis.
func checkDirectoryOptions(opt Options) error {
	if opt.PreparedDigest != "" || opt.Analysis != nil || opt.Execution != nil {
		return invalid("prepared options require their prepared features; use RenderFeaturesDirTo")
	}
	return nil
}
