package report

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"path"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/qatoolist/cucumber-reporter/cucumber"
	"github.com/qatoolist/cucumber-reporter/internal/evidence"

	modelmeta "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"

	input "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

// SummaryVersion is the portable run-summary and history schema version.
const SummaryVersion = 4

// RunConfig enables optional analysis. Zero values require no historical input.
type RunConfig struct {
	// ID identifies this run execution; reuse it only when reprocessing the same artifact.
	ID string `json:"id,omitempty"`
	// Environment scopes history and comparisons (for example staging).
	Environment string `json:"environment,omitempty"`
	// Branch scopes history and comparisons alongside Environment.
	Branch string `json:"branch,omitempty"`
	// Revision is the source-control revision supplied by the caller or runner.
	Revision string `json:"revision,omitempty"`
	// RecordedAt is an RFC3339 timestamp, including its timezone.
	RecordedAt string `json:"recordedAt,omitempty"`
	// ExpectedScenarios is the declared scenario count; zero leaves it unspecified.
	ExpectedScenarios int `json:"expectedScenarios,omitempty"`
	// ExpectedShards lists the logical input names required for completeness.
	ExpectedShards []string `json:"expectedShards,omitempty"`
	// Metadata supplies scenario identity, timing and attempts keyed by navigation ID.
	Metadata map[string]ScenarioMeta `json:"metadata,omitempty"`
	// Baseline is the optional earlier summary used for comparison gates.
	Baseline *RunSummary `json:"baseline,omitempty"`
	// History contains prior summaries; omission disables historical analysis.
	History []RunSummary `json:"history,omitempty"`
	// Retain is the maximum snapshots per environment/branch; zero defaults to 30.
	Retain int `json:"retain,omitempty"`
	// Gates enables explicit quality checks; the zero value enables none.
	Gates GatePolicy `json:"gates,omitzero"`
	// Redaction removes configured evidence secrets before rendering and export.
	Redaction *Redaction `json:"redaction,omitempty"`
	// Issues supplies optional lifecycle constraints for known-issue annotations.
	Issues []IssuePolicy `json:"issues,omitempty"`
	// Fingerprint controls normalized failure grouping.
	Fingerprint FingerprintPolicy `json:"fingerprint,omitzero"`
	// Phases lists measured RFC3339 intervals; durations are not inferred.
	Phases []Phase `json:"phases,omitempty"`
	// Bundle includes attachments and portable exports in report.zip.
	Bundle bool `json:"bundle,omitempty"`
}

// ScenarioMeta supplies identity and measured execution facts; keys are navigation IDs.
type ScenarioMeta struct {
	Identity string    `json:"identity,omitempty"`
	Worker   string    `json:"worker,omitempty"`
	Started  string    `json:"started,omitempty"`
	Finished string    `json:"finished,omitempty"`
	Attempts []Attempt `json:"attempts,omitempty"`
}

// Attempt preserves one execution, including its original evidence.
type Attempt struct {
	Number   int             `json:"number"`
	Status   string          `json:"status"`
	Started  string          `json:"started,omitempty"`
	Finished string          `json:"finished,omitempty"`
	Worker   string          `json:"worker,omitempty"`
	Steps    []cucumber.Step `json:"steps,omitempty"`
}

// Phase is a measured interval, never inferred from summed step durations.
type Phase struct {
	Name     string `json:"name"`
	Worker   string `json:"worker,omitempty"`
	Started  string `json:"started"`
	Finished string `json:"finished"`
}

// IssuePolicy adds optional lifecycle constraints to an existing annotation.
type IssuePolicy struct {
	Ref          string   `json:"ref"`
	Owner        string   `json:"owner,omitempty"`
	Expires      string   `json:"expires,omitempty"`
	ReviewDate   string   `json:"reviewDate,omitempty"`
	Environments []string `json:"environments,omitempty"`
	ErrorPattern string   `json:"errorPattern,omitempty"`
}

// FingerprintPolicy controls explainable error grouping. Patterns are applied in order.
type FingerprintPolicy struct {
	IncludeStep bool     `json:"includeStep,omitempty"`
	StackLines  int      `json:"stackLines,omitempty"`
	Patterns    []string `json:"patterns,omitempty"`
}

// GatePolicy is opt-in. Missing baseline/completeness information fails requested checks.
type GatePolicy struct {
	// Flaky rejects a passing final attempt after a non-passing execution.
	Flaky bool `json:"flaky,omitempty"`
	// Empty rejects a run with no executed scenarios.
	Empty bool `json:"empty,omitempty"`
	// Pending rejects pending scenarios, matching a strict runner policy.
	Pending bool `json:"pending,omitempty"`
	// Unexpected fails the gate on unexpected failed outcomes, including run hooks.
	Unexpected bool `json:"unexpected,omitempty"`
	// Undefined fails on undefined or ambiguous execution outcomes.
	Undefined bool `json:"undefined,omitempty"`
	// Incomplete fails when expected shard or scenario completeness is unmet.
	Incomplete bool `json:"incomplete,omitempty"`
	// NewFailures fails on failures newly observed against the baseline.
	NewFailures bool `json:"newFailures,omitempty"`
	// MinRegressionMs is the minimum duration regression in milliseconds; nil disables it.
	MinRegressionMs *float64 `json:"minRegressionMs,omitempty"`
	// MaxRegressionPercent is the allowed percentage increase against comparable baseline duration.
	MaxRegressionPercent float64 `json:"maxRegressionPercent,omitempty"`
	// AllowFirstRun permits comparison checks when no baseline is supplied.
	AllowFirstRun bool `json:"allowFirstRun,omitempty"`
}

// IssueObservation preserves a known-issue state for lifecycle review across scoped runs.
type IssueObservation struct {
	GeneratedIdentity     bool   `json:"generatedIdentity,omitempty"`
	Ref                   string `json:"ref"`
	Identity              string `json:"identity"`
	State                 string `json:"state"`
	ConsecutiveUnobserved int    `json:"consecutiveUnobserved,omitempty"`
}

// RunSummary is the versioned offline comparison and history snapshot.
type RunSummary struct {
	// RunnerFailed preserves an explicit runner failure signal.
	RunnerFailed bool `json:"runnerFailed,omitempty"`
	// GeneratedID distinguishes tool-generated execution IDs from caller metadata.
	GeneratedID bool `json:"generatedID,omitempty"`
	// ObservedScenarios excludes synthetic run-hook records.
	ObservedScenarios int `json:"observedScenarios,omitempty"`
	// RunHooks counts synthetic run-hook executions separately from scenarios.
	RunHooks int `json:"runHooks,omitempty"`
	// IssueRecords stores observed known-issue lifecycle state for history.
	IssueRecords []IssueObservation `json:"issueRecords,omitempty"`
	// Version is the portable summary schema version.
	Version int `json:"version"`
	// ID identifies this execution; reuse it only when reprocessing the same artifact.
	ID string `json:"id"`
	// Name is the display name of the run or scenario.
	Name string `json:"name,omitempty"`
	// Environment scopes history and comparisons (for example staging).
	Environment string `json:"environment,omitempty"`
	// Branch scopes history and comparisons alongside Environment.
	Branch string `json:"branch,omitempty"`
	// Revision is the source-control revision supplied by the caller or runner.
	Revision string `json:"revision,omitempty"`
	// RecordedAt is an RFC3339 timestamp, including its timezone.
	RecordedAt string `json:"recordedAt,omitempty"`
	// Counts includes scenario outcomes. RunHooks records synthetic hook executions separately.
	Counts    map[string]int    `json:"counts"`
	Scenarios []ScenarioSummary `json:"scenarios"`
	// Completeness is complete, incomplete or unknown based on supplied expectations.
	Completeness string `json:"completeness,omitempty"`
	// MissingShards lists expected inputs that were absent.
	MissingShards []string `json:"missingShards,omitempty"`
	// ExpectedScenarios is the declared scenario count; zero leaves it unspecified.
	ExpectedScenarios int `json:"expectedScenarios,omitempty"`
	// Computed is the verdict calculated from execution outcomes.
	Computed Verdict `json:"computed,omitzero"`
	// VerdictSource identifies whether the displayed headline is computed or caller-supplied.
	VerdictSource string `json:"verdictSource,omitempty"`
	// Comparison describes changes relative to the optional baseline.
	Comparison *Comparison `json:"comparison,omitempty"`
	// GateFailures contains safe explanations for each failed requested gate.
	GateFailures []string `json:"gateFailures,omitempty"`
	// Diagnostics contains safe numeric locations and explanatory notices.
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
	// History contains prior summaries; omission disables historical analysis.
	History []HistoryPoint `json:"history,omitempty"`
	// Issues supplies optional lifecycle constraints for known-issue annotations.
	Issues []IssuePolicy `json:"issues,omitempty"`
	// Phases lists measured RFC3339 intervals; durations are not inferred.
	Phases []Phase `json:"phases,omitempty"`
}

// ScenarioSummary contains no attachment bodies; attempt evidence stays in the HTML.
type ScenarioSummary struct {
	// GeneratedIdentity distinguishes generated comparison hashes from explicit metadata.
	GeneratedIdentity bool `json:"generatedIdentity,omitempty"`
	// SyntheticURI marks a tool-generated location rather than a source path.
	SyntheticURI bool `json:"syntheticURI,omitempty"`
	// RunHook marks synthetic hook evidence; it is excluded from expected scenario counts.
	RunHook bool `json:"runHook,omitempty"`
	// RerunUnavailable indicates that redaction prevents a usable source selector.
	RerunUnavailable bool `json:"rerunUnavailable,omitempty"`
	// ID is the scenario navigation ID within this report, not its historical identity.
	ID string `json:"id,omitempty"`
	// Identity is the comparison identity, distinct from the navigation ID.
	Identity string `json:"identity"`
	// Name is the display name of the run or scenario.
	Name string `json:"name,omitempty"`
	// URI is the normalized feature source path.
	URI string `json:"uri,omitempty"`
	// Line is the source location, or zero when the runner supplied none.
	Line int `json:"line,omitempty"`
	// Status is the computed effective outcome, including known-issue classification.
	Status string `json:"status"`
	// Duration is the summed step duration in nanoseconds, not wall-clock time.
	Duration int64 `json:"duration"`
	// Tags preserves scenario tags supplied by the runner.
	Tags []string `json:"tags,omitempty"`
	// Sources lists the logical shards that supplied this scenario.
	Sources []string `json:"sources,omitempty"`
	// Worker identifies the execution worker supplied by the runner.
	Worker string `json:"worker,omitempty"`
	// Started is an RFC3339Nano execution start timestamp, when measured.
	Started string `json:"started,omitempty"`
	// Finished is an RFC3339Nano execution end timestamp, when measured.
	Finished string `json:"finished,omitempty"`
	// Attempts retains executions in attempt order, with the final one authoritative.
	Attempts []AttemptSummary `json:"attempts,omitempty"`
	// FirstStatus is the first attempt outcome before retries.
	FirstStatus string `json:"firstStatus,omitempty"`
	// Retried indicates that more than one attempt was observed.
	Retried bool `json:"retried,omitempty"`
	// Samples counts distinct executions observed in this environment and branch.
	Samples int `json:"samples,omitempty"`
	// Failures counts unexpected failures among Samples.
	Failures int `json:"failures,omitempty"`
	// Fingerprint is the normalized error-group hash, or empty when no error is grouped.
	Fingerprint string `json:"fingerprint,omitzero"`
	// GroupReason explains the failure grouping decision.
	GroupReason string `json:"groupReason,omitempty"`
	// Change is the baseline comparison classification, when comparable.
	Change string `json:"change,omitempty"`
	// DurationComparable indicates that a baseline duration is available.
	DurationComparable bool `json:"durationComparable,omitempty"`
	// DurationDelta is the signed duration difference in nanoseconds.
	DurationDelta int64 `json:"durationDelta,omitempty"`
}

// AttemptSummary retains outcome/timing without duplicating evidence into history.
type AttemptSummary struct {
	Number   int    `json:"number"`
	Status   string `json:"status"`
	Started  string `json:"started,omitempty"`
	Finished string `json:"finished,omitempty"`
	Worker   string `json:"worker,omitempty"`
}

// HistoryPoint is one run's scoped aggregate, embedded for offline viewing.
type HistoryPoint struct {
	GeneratedID bool           `json:"generatedID,omitempty"`
	ID          string         `json:"id"`
	RecordedAt  string         `json:"recordedAt,omitempty"`
	Counts      map[string]int `json:"counts"`
	Duration    int64          `json:"duration"`
}

// Comparison identifies its baseline and explicitly calls absent tests missing.
type Comparison struct {
	Baseline     string   `json:"baseline"`
	NewFailures  []string `json:"newFailures"`
	Fixed        []string `json:"fixed"`
	Missing      []string `json:"missing"`
	Added        []string `json:"added"`
	NewlySkipped []string `json:"newlySkipped"`
	Slower       []string `json:"slower"`
}

// Diagnostic contains only safe locations and fixed explanations, never input values.
type Diagnostic struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// Input is one explicitly named shard. Duplicate scenarios are rejected, not silently merged.
type runInput struct {
	// Attempts supplies parsed Messages executions; preparePublicRun owns final selection and metadata binding.
	Attempts []input.MessageAttempt
	Name     string
	Features []input.Feature
	// Diagnostics are validated caller diagnostics, published after the
	// reporter's own and redacted with the rest of the summary.
	Diagnostics []Diagnostic
}

// preparedRun is an immutable-by-convention render/export snapshot.
type preparedRun struct {
	Features []input.Feature
	Options  renderOptions
	Summary  RunSummary
	History  []RunSummary
	Metadata map[string]ScenarioMeta
	Bundle   bool
	// OmitHistory means the optional archive cannot fit its reader budget.
	OmitHistory bool
}

// Identity returns "test-" followed by 32 lowercase hex digits: the first 16 bytes
// of SHA-256 over normalized URI, a NUL separator and the identity component.
// URI normalization replaces backslashes with slashes and cleans dot segments.
// PrepareRun selects a unique name, a colliding-name/line component or a synthetic
// hook component; explicit metadata identities can override that selection.
func Identity(uri, name string) string {
	uri = path.Clean(strings.ReplaceAll(uri, "\\", "/"))
	sum := sha256.Sum256([]byte(uri + "\x00" + name))
	return "test-" + hex.EncodeToString(sum[:16])
}

func failedOutcome(s string) bool { return s == "failed" || s == "undefined" || s == "ambiguous" }

// preparePublicRun merges inputs, applies optional policies, then computes all exports from one model.
func preparePublicRun(inputs []runInput, opt renderOptions, cfg RunConfig) (*preparedRun, error) {
	return prepareRunWithBudget(inputs, opt, cfg, HistoryByteLimit)
}

func prepareRunWithBudget(inputs []runInput, opt renderOptions, cfg RunConfig, historyBudget int) (*preparedRun, error) {
	if cfg.Gates.MinRegressionMs != nil && (*cfg.Gates.MinRegressionMs < 0 || math.IsNaN(*cfg.Gates.MinRegressionMs) || math.IsInf(*cfg.Gates.MinRegressionMs, 0) || *cfg.Gates.MinRegressionMs > float64(math.MaxInt64)/1e6) {
		return nil, invalid("gates.minRegressionMs must be finite, nonnegative and no greater than %.0f", float64(math.MaxInt64)/1e6)
	}
	if len(inputs) == 0 {
		return nil, input.ErrNoResults
	}
	if cfg.Retain < 0 || cfg.ExpectedScenarios < 0 || cfg.Gates.MaxRegressionPercent < 0 || math.IsNaN(cfg.Gates.MaxRegressionPercent) || math.IsInf(cfg.Gates.MaxRegressionPercent, 0) {
		return nil, invalid("run limits")
	}
	// Configuration contains caller-owned history, policies and nested attempt evidence.
	originalMetadata := cfg.Metadata
	cfg.Metadata = nil
	configJSON, cloneErr := json.Marshal(cfg)
	if cloneErr != nil {
		return nil, cloneErr
	}
	cfg = RunConfig{}
	if cloneErr = json.Unmarshal(configJSON, &cfg); cloneErr != nil {
		return nil, cloneErr
	}
	cfg.Metadata, cloneErr = cloneMetadata(originalMetadata)
	if cloneErr != nil {
		return nil, cloneErr
	}
	if cfg.Baseline != nil {
		if e := validateSummary(*cfg.Baseline); e != nil {
			return nil, fmt.Errorf("baseline: %w", e)
		}
	}
	for i, s := range cfg.History {
		if e := validateSummary(s); e != nil {
			return nil, fmt.Errorf("history[%d]: %w", i, e)
		}
	}
	// Issue policies are decided on the unmasked environment and policies; the
	// redaction below masks only what is published, including summary.Issues.
	policyCfg := cfg
	policyCfg.Issues = make([]IssuePolicy, len(cfg.Issues))
	for i, p := range cfg.Issues {
		p.Environments = slices.Clone(p.Environments)
		policyCfg.Issues[i] = p
	}
	runIDRedacted := false
	if !emptyRedaction(cfg.Redaction) {
		r, e := newRedactor(cfg.Redaction)
		if e != nil {
			return nil, e
		}
		runIDRedacted = r.text(cfg.ID) != cfg.ID
		// A stored summary is in scope when its environment and branch equal the
		// current ones, unmasked or (for summaries stored redacted) masked. Refuse
		// one that only masking brings into scope.
		masked := scopeKey(RunSummary{Environment: r.text(cfg.Environment), Branch: r.text(cfg.Branch)})
		current := scopeKey(RunSummary{Environment: cfg.Environment, Branch: cfg.Branch})
		// Two different unmasked scopes must not merge into one masked scope.
		rawScope := map[string]string{masked: current}
		for i, s := range cfg.History {
			key, raw := scopeKey(RunSummary{Environment: r.text(s.Environment), Branch: r.text(s.Branch)}), scopeKey(s)
			if raw == key {
				continue // stored already redacted
			}
			if seen, ok := rawScope[key]; ok && seen != raw {
				return nil, invalid("redaction makes history[%d] scope match another scope only when masked; use nonsecret environment and branch names", i)
			}
			rawScope[key] = raw
		}
		if b := cfg.Baseline; b != nil && scopeKey(*b) != current && scopeKey(*b) != masked {
			return nil, invalid("baseline scope differs from explicitly configured environment or branch")
		}
		cfg.ID = r.text(cfg.ID)
		cfg.Environment = r.text(cfg.Environment)
		cfg.Branch = r.text(cfg.Branch)
		cfg.Revision = r.text(cfg.Revision)
		r.strings(reflect.ValueOf(cfg.History))
		r.strings(reflect.ValueOf(cfg.Baseline))
		for i := range cfg.Issues {
			for j := range cfg.Issues[i].Environments {
				cfg.Issues[i].Environments[j] = r.text(cfg.Issues[i].Environments[j])
			}
		}
		r.strings(reflect.ValueOf(cfg.Phases))
	}
	if cfg.Retain == 0 {
		cfg.Retain = 30
	}
	if cfg.Retain > 1000 {
		return nil, invalid("history retention exceeds 1000")
	}
	for _, p := range cfg.Fingerprint.Patterns {
		if _, err := regexp.Compile(p); err != nil {
			return nil, invalid("fingerprint pattern")
		}
	}
	if cfg.Fingerprint.StackLines < 0 || cfg.Fingerprint.StackLines > 50 {
		return nil, invalid("fingerprint stackLines")
	}
	if cfg.RecordedAt != "" {
		if _, err := time.Parse(time.RFC3339Nano, cfg.RecordedAt); err != nil {
			return nil, invalid("recordedAt")
		}
	}
	var sourceFeatures []input.Feature
	for _, in := range inputs {
		sourceFeatures = append(sourceFeatures, in.Features...)
		for _, a := range in.Attempts {
			sourceFeatures = append(sourceFeatures, input.Feature{URI: a.URI, Elements: []input.Scenario{a.Scenario}})
		}
	}
	if err := rejectRepeatedInputs(inputs); err != nil {
		return nil, err
	}
	rawInputs := rawStreams(inputs)
	if err := evidence.CheckInputs(rawInputs); err != nil {
		return nil, err
	}
	floor := modelmeta.Evidence(sourceFeatures)
	runnerFailed := false
	fromMessages := make([]bool, len(inputs))
	for i, in := range inputs {
		fromMessages[i] = len(in.Attempts) > 0
	}
	inputs, runnerMetadata, mergeErr := mergeInputAttempts(inputs)
	if mergeErr != nil {
		return nil, mergeErr
	}
	for _, in := range inputs {
		runnerFailed = runnerFailed || modelmeta.RunnerFailed(in.Features)
	}
	var features []input.Feature
	featureIndex, featureInput := map[string]int{}, map[string]int{}
	shardNames := map[string]bool{}
	sourceByLocation := map[string][]string{}
	var inputDiagnostics []Diagnostic

	for i, in := range inputs {
		if in.Name == "" || shardNames[in.Name] {
			return nil, invalidInput("inputs[%d] needs a unique Name", i)
		}
		shardNames[in.Name] = true
		if len(in.Features) == 0 {
			continue
		}
		if err := input.Validate(in.Features); err != nil {
			return nil, err
		}
		for inputFeatureIndex, f := range in.Features {
			uri := path.Clean(strings.ReplaceAll(f.URI, "\\", "/"))
			fi, exists := featureIndex[uri]
			if !exists {
				fi = len(features)
				featureIndex[uri] = fi
				featureInput[uri] = i
				nf := f
				nf.Elements = nil
				features = append(features, nf)
			} else if features[fi].Name != f.Name || features[fi].Description != f.Description || !slices.Equal(features[fi].Tags, f.Tags) {
				if fromMessages[i] != fromMessages[featureInput[uri]] {
					return nil, invalidInput("inputs[%d] feature %q: name, description or tags differ from inputs[%d]; mixed JSON and Messages shards are not supported", i, uri, featureInput[uri])
				}
				return nil, invalidInput("inputs[%d] feature %q: name, description or tags conflict with inputs[%d]; shards of one run must carry the same feature metadata", i, uri, featureInput[uri])
			}
			var background []input.Step
			backgroundIndex := -1
			for elementIndex, sc := range f.Elements {
				if strings.EqualFold(sc.Type, "background") {
					backgroundIndex = elementIndex
					background = append(background, sc.Before...)
					background = append(background, sc.Steps...)
					background = append(background, sc.After...)
					continue
				}
				var foldErr error
				sc, foldErr = foldScenarioSteps(sc, background)
				if foldErr != nil {
					return nil, foldErr
				}
				background = nil
				backgroundIndex = -1
				for _, st := range sc.Steps {
					for _, a := range st.Embeddings {
						if int64(len(a.Data)) > opt.Limits.WithDefaults().Embedding {
							return nil, input.ErrTooLarge
						}
					}
				}
				loc := sourceLocation(uri, sc)

				if sources := sourceByLocation[loc]; len(sources) > 0 {
					if sources[0] != in.Name {
						return nil, invalidInput("inputs %q and %q repeat %q:%d; merging reruns or matrix cells across inputs is not supported in v0.3.0", sources[0], in.Name, uri, sc.Line)
					}
					return nil, invalidInput("input %q repeats %q:%d; repeated executions were not recognized as an in-file retry", in.Name, uri, sc.Line)
				}
				sourceByLocation[loc] = []string{in.Name}
				features[fi].Elements = append(features[fi].Elements, sc)
			}
			if backgroundIndex >= 0 {
				nonpassed := false
				for _, st := range background {
					nonpassed = nonpassed || st.Result.Status != input.StatusPassed
				}
				if nonpassed {
					orphan := f.Elements[backgroundIndex]
					orphan.Type = "orphan_background"
					orphan.Before = nil
					orphan.After = nil
					orphan.Steps = background
					features[fi].Elements = append(features[fi].Elements, orphan)
				}
				modelmeta.FeatureMetadata(&features[fi]).OrphanBackground = true
				inputDiagnostics = append(inputDiagnostics, Diagnostic{Path: fmt.Sprintf("input[%d].feature[%d].elements[%d]", i, inputFeatureIndex, backgroundIndex), Message: "Background has no following scenario; non-passing evidence remains countable"})
			}
		}
	}
	features = slices.DeleteFunc(features, func(f input.Feature) bool {
		return len(f.Elements) == 0 && !modelmeta.FeatureMetadata(&f).OrphanBackground
	})
	// validateModel the complete execution model, including hooks supplied by Go callers.
	if err := validateRenderFeatures(features); err != nil {
		return nil, err
	}
	if floorErr := evidence.Check(floor, modelmeta.Evidence(features)); floorErr != nil {
		return nil, floorErr
	}
	// Reconcile runner ownership against the final merged evidence, not each input's
	// pre-merge status. An unrelated pending case cannot be rescued by another case.
	finalStatuses := map[string]string{}
	for _, f := range features {
		for _, sc := range f.Elements {
			finalStatuses[sourceLocation(f.URI, sc)] = string(modelmeta.Decide(sc, nil, nil).Status)
		}
	}
	finalRaw := finalEvidence(features, finalStatuses)
	runnerFailed = false
	for _, stream := range rawInputs {
		runnerFailed = runnerFailed || unresolvedStream(stream, finalRaw)
	}
	for _, f := range sourceFeatures {
		for _, sc := range f.Elements {
			claim, exists := finalRaw[sourceLocation(f.URI, sc)]
			if err := evidence.CheckCase(modelmeta.ScenarioMetadata(&sc).RawCaseAudit, claim, exists); err != nil {
				return nil, fmt.Errorf("%s:%d: %w", f.URI, sc.Line, err)
			}
			if err := evidence.CheckRunner(modelmeta.ScenarioMetadata(&sc).RawRunnerAudit, runnerFailed); err != nil {
				return nil, err
			}
		}
	}
	if err := evidence.CheckOutcomes(rawInputs, finalRaw, runnerFailed); err != nil {
		return nil, err
	}
	for i := range features {
		for j := range features[i].Elements {
			modelmeta.ScenarioMetadata(&features[i].Elements[j]).RunnerFailed = runnerFailed
		}
	}
	// Work on a deep copy so redaction and policy application cannot mutate caller data.
	features, err := modelmeta.CloneFeatures(features)
	if err != nil {
		return nil, err
	}
	metadata := cfg.Metadata

	slices.SortFunc(features, func(a, b input.Feature) int {
		return strings.Compare(normalizedURI(a.URI), normalizedURI(b.URI))
	})
	setRunNavigation(features)
	ids := scenarioIDs(features)
	validIDs := map[string]bool{}
	identities := map[string]bool{}
	sourcesByID := map[string][]string{}
	explicitIdentity := map[string]bool{}
	originalURIs := map[string]string{}
	collisionIDs := map[string]bool{}
	for fi, f := range features {
		counts := map[string]int{}
		for _, sc := range f.Elements {
			counts[sc.Name]++
		}
		for si, sc := range f.Elements {
			modelmeta.ScenarioMetadata(&features[fi].Elements[si]).IdentityNameCount = max(modelmeta.ScenarioMetadata(&sc).IdentityNameCount, counts[sc.Name])
		}
	}
	for fi, f := range features {
		seenLines := map[int]bool{}
		for si, sc := range f.Elements {
			id := ids[fi][si]
			originalURIs[id] = f.URI
			if sc.Line > 0 && seenLines[sc.Line] {
				return nil, invalidInput("%q:%d: enhanced scenarios need distinct source lines", f.URI, sc.Line)
			}
			seenLines[sc.Line] = true
			validIDs[id] = true
			sourcesByID[id] = sourceByLocation[sourceLocation(f.URI, sc)]
			m := metadata[id]
			if runner, ok := runnerMetadata[sourceLocation(f.URI, sc)]; ok {
				if len(m.Attempts) > 0 && len(runner.Attempts) > 1 {
					return nil, invalid("attempts supplied by both runner and metadata")
				}
				if len(m.Attempts) == 0 {
					m.Attempts = runner.Attempts
				}
				if m.Worker == "" {
					m.Worker = runner.Worker
				}
				if m.Started == "" && m.Finished == "" {
					m.Started = runner.Started
					m.Finished = runner.Finished
				}
			}
			identity := m.Identity
			explicitIdentity[id] = identity != ""
			if identity == "" {
				identity = scenarioIdentity(f.URI, sc)
				collisionIDs[id] = modelmeta.ScenarioMetadata(&sc).IdentityNameCount > 1 && sc.Type != "run_hook"
			}
			if identities[identity] {
				return nil, invalid("ambiguous scenario identity; supply distinct metadata identities")
			}
			identities[identity] = true
			if metadata == nil {
				metadata = map[string]ScenarioMeta{}
			}
			m.Identity = identity
			for i := range m.Attempts {
				folded, foldErr := foldScenarioSteps(input.Scenario{Type: sc.Type, Steps: internalSteps(m.Attempts[i].Steps, 0)}, nil)
				if foldErr != nil {
					return nil, foldErr
				}
				m.Attempts[i].Steps = publicSteps(folded.Steps, 0)
			}
			if err = validateMeta(m, sc); err != nil {
				return nil, err
			}
			state := modelmeta.ScenarioMetadata(&features[fi].Elements[si])
			if len(m.Attempts) > 1 && len(state.PreviousAttempts) == 0 {
				var prior []modelmeta.ExecutionEvidence
				for _, a := range m.Attempts[:len(m.Attempts)-1] {
					status := input.Status(a.Status)
					state.ReportedAttempts = append(state.ReportedAttempts, status)
					prior = append(prior, modelmeta.ExecutionEvidence{Own: true, Results: []input.Status{status}})
				}
				state.OutcomeFloor = append(prior, state.OutcomeFloor...)
			}
			metadata[id] = m
		}
	}
	observedCount := 0
	for _, f := range features {
		for _, sc := range f.Elements {
			if sc.Type != "run_hook" {
				observedCount++
			}
		}
	}
	missingExpected := cfg.ExpectedScenarios > observedCount
	if len(cfg.ExpectedShards) > 0 {
		expected := map[string]bool{}
		for i, name := range cfg.ExpectedShards {
			if expected[name] {
				return nil, invalid("expectedShards[%d] repeats a name", i)
			}
			expected[name] = true
		}
		for i, in := range inputs {
			if !expected[in.Name] {
				return nil, invalid("inputs[%d] is not listed in expectedShards; correct the expected shard names", i)
			}
		}
	}
	for _, name := range cfg.ExpectedShards {
		if !shardNames[name] {
			missingExpected = true
		}
	}
	unmatchedMetadata := 0
	for id := range metadata {
		if !validIDs[id] {
			if !missingExpected {
				return nil, invalid("input metadata names no included scenario and no configured expectation establishes missing execution")
			}
			delete(metadata, id)
			unmatchedMetadata++
		}
	}
	opt.Annotations = slices.Clone(opt.Annotations)
	ownMaps := map[string]map[int]int{}
	for fi, f := range features {
		for si, sc := range f.Elements {
			ownMaps[ids[fi][si]] = ownStepMap(sc)
		}
	}
	for i := range opt.Annotations {
		a := &opt.Annotations[i]
		stepMap, ok := ownMaps[a.Scenario]
		if !ok {
			continue
		} // Existing option validation supplies the safe unknown-scenario error.
		validateStep := func(index int) error {
			if index == 0 {
				return nil
			}
			if _, ok := stepMap[index]; !ok {
				return invalid("annotations[%d].step is outside the scenario's own steps", i)
			}
			return nil
		}
		a.Outcomes = slices.Clone(a.Outcomes)
		if err = validateStep(a.Step); err != nil {
			return nil, err
		}
		for j := range a.Outcomes {
			if err = validateStep(a.Outcomes[j].Step); err != nil {
				return nil, err
			}
		}
	}
	diagnostics, err := applyIssuePolicies(features, ids, &opt, policyCfg)
	if err != nil {
		return nil, err
	}
	if unmatchedMetadata > 0 {
		diagnostics = append(diagnostics, Diagnostic{Path: "metadata", Message: fmt.Sprintf("%d metadata entries have no scenario while configured expectations establish missing execution", unmatchedMetadata)})
	}
	// validateModel original identity ambiguity first, then recompute automatic identities
	// from redacted display data so removed secrets do not leave guessable hashes.
	// Known-issue coverage and issue policies are decided on the original evidence.
	// Snapshot it before masking: redaction must neither widen nor narrow coverage,
	// and the floor checks the decision against these unmasked values.
	var original *originalEvidence
	var declared map[int]string
	if !emptyRedaction(cfg.Redaction) {
		if original, err = snapshotEvidence(features, ids, opt.Annotations); err != nil {
			return nil, err
		}
		declared = declaredStepNames(features, ids, opt.Annotations)
	}
	var retryPairs []retryStepPair
	if !emptyRedaction(cfg.Redaction) {
		retryPairs = differingRetrySteps(features, ids, metadata)
	}
	matched := map[int]bool{}
	for i, name := range declared {
		matched[i] = clean(opt.Annotations[i].StepText) == name
	}
	rawIdentity := map[string]string{}
	for id, m := range metadata {
		if explicitIdentity[id] {
			rawIdentity[id] = m.Identity
		}
	}
	if cfg.Redaction != nil {
		if err = redactRun(features, &opt, metadata, explicitIdentity, cfg.Redaction); err != nil {
			return nil, err
		}
	}
	// Earlier attempts are matched to the final steps by keyword and name; keep a
	// step that differed before masking distinct after it.
	for _, pair := range retryPairs {
		sc := &features[pair.feature].Elements[pair.scenario]
		cur := sc.Steps[pair.current]
		m := metadata[ids[pair.feature][pair.scenario]]
		prior := &m.Attempts[pair.attempt].Steps[pair.step]
		previous := modelmeta.ScenarioMetadata(sc).PreviousAttempts
		for prior.Name == cur.Name && prior.Keyword == cur.Keyword {
			prior.Name += " (differs before redaction)"
		}
		if pair.attempt < len(previous) && pair.step < len(previous[pair.attempt].Steps) {
			previous[pair.attempt].Steps[pair.step].Name = prior.Name
		}
	}
	// Carry the unmasked step-text decision: masking must not make a mismatch
	// match, nor a match mismatch. The published StepText is masked either way.
	for i, name := range declaredStepNames(features, ids, opt.Annotations) {
		if _, ok := declared[i]; !ok {
			continue
		}
		a := &opt.Annotations[i]
		switch {
		case matched[i] && clean(a.StepText) != name:
			a.StepText = name
		case !matched[i] && clean(a.StepText) == name:
			a.StepText = name + " (differs before redaction)"
		}
	}
	if !emptyRedaction(cfg.Redaction) {
		redactor, _ := newRedactor(cfg.Redaction)
		for fi, f := range features {
			counts := map[string]int{}
			for name, n := range modelmeta.FeatureMetadata(&f).IdentityCatalog {
				counts[redactor.text(name)] += n
			}
			modelmeta.FeatureMetadata(&features[fi]).IdentityCatalog = counts
			observed := map[string]int{}
			for _, sc := range f.Elements {
				observed[sc.Name]++
			}
			for si, sc := range f.Elements {
				count := max(counts[sc.Name], observed[sc.Name])
				modelmeta.ScenarioMetadata(&features[fi].Elements[si]).IdentityNameCount = count
				id := ids[fi][si]
				collisionIDs[id] = !explicitIdentity[id] && count > 1 && sc.Type != "run_hook"
			}
		}
	}
	maskedIdentities := 0
	if cfg.Redaction != nil {
		identities = map[string]bool{}
		for fi, f := range features {
			for si, sc := range f.Elements {
				id := ids[fi][si]
				m := metadata[id]
				if !explicitIdentity[id] {
					if masked := scenarioIdentity(f.URI, sc); masked != m.Identity {
						maskedIdentities++
						m.Identity = masked
					}
				} else if m.Identity != rawIdentity[id] {
					maskedIdentities++
				}
				if identities[m.Identity] {
					return nil, invalid("redaction makes scenario identities ambiguous; supply opaque metadata identities")
				}
				identities[m.Identity] = true
				metadata[id] = m
			}
		}
	}
	// Redacted paths may change navigation slugs; remap caller references while keeping historical identity.
	setRunNavigation(features)
	newIDs := scenarioIDs(features)
	remapped := map[string]ScenarioMeta{}
	remappedExplicit := map[string]bool{}
	remappedSources := map[string][]string{}
	rerunUnavailable := map[string]bool{}
	navigation := map[string]string{}
	for fi := range ids {
		for si, oldID := range ids[fi] {
			newID := newIDs[fi][si]
			navigation[oldID] = newID
			remapped[newID] = metadata[oldID]
			remappedExplicit[newID] = explicitIdentity[oldID]
			rerunUnavailable[newID] = originalURIs[oldID] != features[fi].URI
			remappedSources[newID] = sourcesByID[oldID]
		}
	}
	for ai := range opt.Annotations {
		if id, ok := navigation[opt.Annotations[ai].Scenario]; ok {
			opt.Annotations[ai].Scenario = id
		}
	}
	original.remap(navigation)
	var collisionRefs []string
	for _, row := range ids {
		for _, id := range row {
			if collisionIDs[id] {
				collisionRefs = append(collisionRefs, navigation[id])
			}
		}
	}
	if len(collisionRefs) > 0 {
		diagnostics = append(diagnostics, Diagnostic{Path: "identity", Message: "identity follows the line for " + strings.Join(collisionRefs, ", ") + "; set metadata.<navID>.identity using the pre-redaction navigation ID (runScenarioIDs) to pin it"})
	}
	explicitIdentity = remappedExplicit
	ids = newIDs
	metadata = remapped
	for fi, f := range features {
		for si, sc := range f.Elements {
			id := newIDs[fi][si]
			m := metadata[id]
			n := len(m.Attempts)
			if n > 0 && reflect.DeepEqual(m.Attempts[n-1].Steps, publicSteps(sc.Steps, 0)) {
				m.Attempts[n-1].Steps = nil
				metadata[id] = m
			}
		}
	}
	for fi, f := range features {
		for si, sc := range f.Elements {
			if modelmeta.ScenarioMetadata(&sc).InferredRetryFailure {
				diagnostics = append(diagnostics, Diagnostic{Path: fmt.Sprintf("feature[%d].elements[%d]", fi, si), Message: "Runner retried an execution with no reported failed step; the earlier attempt retains an unattributed runner failure"})
			}
			if modelmeta.ScenarioMetadata(&sc).CaseAttachments {
				diagnostics = append(diagnostics, Diagnostic{Path: fmt.Sprintf("feature[%d].elements[%d]", fi, si), Message: "Attachment references a step outside this case; retained as Around hook output"})
			}
			if modelmeta.ScenarioMetadata(&sc).MissingStepResults {
				diagnostics = append(diagnostics, Diagnostic{Path: fmt.Sprintf("feature[%d].elements[%d]", fi, si), Message: "Runner completed the case without step results; missing execution is represented by a failing runner-evidence hook"})
			}
			if modelmeta.ScenarioMetadata(&sc).BackgroundUnresolved {
				diagnostics = append(diagnostics, Diagnostic{Path: fmt.Sprintf("feature[%d].elements[%d]", fi, si), Message: "Background origins unavailable: pickle steps lack AST references; these steps count as scenario-owned. Include gherkinDocument and astNodeIds"})
			}
		}
	}
	sourcesByID = remappedSources
	opt, err = normalize(opt)
	if err != nil {
		return nil, err
	}
	classified, err := buildPage(features, opt, nil)
	if err != nil {
		return nil, err
	}
	summary := RunSummary{Version: SummaryVersion, ID: cfg.ID, Name: opt.Name, Environment: cfg.Environment, Branch: cfg.Branch, Revision: cfg.Revision, RecordedAt: cfg.RecordedAt, Counts: map[string]int{}, Completeness: "unknown", ExpectedScenarios: cfg.ExpectedScenarios, VerdictSource: "computed", Diagnostics: diagnostics, Issues: cfg.Issues, Phases: cfg.Phases}
	summary.Diagnostics = append(summary.Diagnostics, inputDiagnostics...)
	if opt.Verdict != nil {
		summary.VerdictSource = "caller"
	}
	var all []string
	for fi, f := range features {
		for si, sc := range f.Elements {
			id := ids[fi][si]
			m := metadata[id]
			ss := ScenarioSummary{GeneratedIdentity: !explicitIdentity[id], SyntheticURI: modelmeta.FeatureMetadata(&f).SyntheticRunHooks, RunHook: sc.Type == "run_hook", RerunUnavailable: rerunUnavailable[id], ID: id, Identity: m.Identity, Name: sc.Name, URI: f.URI, Line: sc.Line, Status: classified.Features[fi].Elements[si].Status, Tags: uniqueTags(f.Tags, sc.Tags), Worker: m.Worker, Started: m.Started, Finished: m.Finished}
			for _, st := range sc.Steps {
				if st.Result.Duration != nil {
					ss.Duration = saturatingAdd(ss.Duration, int64(*st.Result.Duration))
				}
			}
			ss.Sources = sourcesByID[id]
			if len(ss.Sources) == 0 {
				ss.Sources = []string{"redacted input"}
			}
			for _, a := range m.Attempts {
				ss.Attempts = append(ss.Attempts, AttemptSummary{a.Number, a.Status, a.Started, a.Finished, a.Worker})
			}
			if len(ss.Attempts) > 0 {
				ss.FirstStatus = ss.Attempts[0].Status
				ss.Retried = len(ss.Attempts) > 1
			}
			ss.Fingerprint, ss.GroupReason = fingerprint(sc, cfg.Fingerprint)
			if modelmeta.Decide(sc, nil, nil).Flaky && !ss.RunHook {
				summary.Counts["flaky"]++
			}
			summary.Scenarios = append(summary.Scenarios, ss)
			if ss.RunHook {
				summary.RunHooks++
			} else {
				summary.ObservedScenarios++
			}
			if !ss.RunHook {
				summary.Counts[ss.Status]++
			}
			all = append(all, ss.Status)
		}
	}
	if summary.ID == "" {
		summary.ID = "run-" + rand.Text()
		summary.GeneratedID = true
	}
	for i, a := range classified.Annotations {
		if a.State == stateInvalid {
			summary.Diagnostics = append(summary.Diagnostics, Diagnostic{Path: fmt.Sprintf("annotations[%d]", i), Message: "Annotation stepText does not match its declared step; it covers no failure"})
		}
		if a.Kind == KindKnown {
			summary.IssueRecords = append(summary.IssueRecords, IssueObservation{GeneratedIdentity: !explicitIdentity[a.Scenario], Ref: a.Ref, Identity: metadata[a.Scenario].Identity, State: a.State})
		}
	}
	summary.RunnerFailed = runnerFailed
	summary.Computed = classified.Verdict
	if runnerFailed {
		if failedExecutions(all) == 0 {
			summary.Diagnostics = append(summary.Diagnostics, Diagnostic{Path: "runner.outcome", Message: "Runner failure could not be attributed to a scenario; the runner may enforce pending, flaky or hook policies absent from its execution events"})
		}
	}

	if cfg.ExpectedScenarios > 0 || len(cfg.ExpectedShards) > 0 {
		summary.Completeness = "complete"
	}
	if cfg.ExpectedScenarios > 0 && summary.ObservedScenarios != cfg.ExpectedScenarios {
		summary.Completeness = "incomplete"
	}
	for _, shard := range cfg.ExpectedShards {
		if !shardNames[shard] {
			summary.MissingShards = append(summary.MissingShards, shard)
			summary.Completeness = "incomplete"
		}
	}
	if cfg.Baseline != nil {
		if err = validateSummary(*cfg.Baseline); err != nil {
			return nil, fmt.Errorf("baseline: %w", err)
		}
		if cfg.Baseline.Environment != cfg.Environment || cfg.Baseline.Branch != cfg.Branch {
			return nil, invalid("baseline scope differs from explicitly configured environment or branch")
		}
		compareRun(&summary, *cfg.Baseline, cfg.Gates)
	}
	// Identities that masking changed (generated from masked names or paths, or
	// explicit identities that contain a match) cannot tell two unmasked values
	// apart, so a baseline or history match on them is masked too.
	if maskedIdentities > 0 && (cfg.Baseline != nil || len(cfg.History) > 0) {
		summary.Diagnostics = append(summary.Diagnostics, Diagnostic{Path: "identity", Message: fmt.Sprintf("%d scenario identities were changed by redaction; baseline and history match them by the masked text, so a changed secret is not a new scenario. Use opaque metadata identities that no redaction pattern matches", maskedIdentities)})
	}
	// Stored summaries are already masked, so scope can only be compared masked.
	if (cfg.Baseline != nil || len(cfg.History) > 0) && (policyCfg.Environment != cfg.Environment || policyCfg.Branch != cfg.Branch) {
		path := "history"
		if cfg.Baseline != nil {
			path = "baseline"
		}
		summary.Diagnostics = append(summary.Diagnostics, Diagnostic{Path: path, Message: "Baseline and history scope was compared on the redacted environment or branch"})
	}
	allHistory := make([]RunSummary, 0, len(cfg.History)+1)
	seenRuns := map[string]bool{}
	for _, old := range cfg.History {
		if err = validateSummary(old); err != nil {
			return nil, fmt.Errorf("history: %w", err)
		}
		key := runKey(old)
		if seenRuns[key] {
			return nil, invalid("duplicate history run")
		}
		seenRuns[key] = true
		if key == runKey(summary) && runIDRedacted {
			return nil, invalid("redacted run id collides with stored history; use a unique nonsecret execution id")
		}
		if key != runKey(summary) {
			allHistory = append(allHistory, old)
		}
	}
	allHistory = append(allHistory, summary)
	archive, historyErr := compactHistory(allHistory, cfg.Retain, historyBudget, scopeKey(summary))
	if historyErr != nil {
		return nil, historyErr
	}
	allHistory = archive.entries
	summary.Diagnostics = append(summary.Diagnostics, archive.diagnostics...)
	var history []RunSummary
	for _, old := range archive.analysis {
		if scopeKey(old) == scopeKey(summary) && old.ID != summary.ID {
			history = append(history, old)
		}
	}
	for _, old := range history {
		hp := HistoryPoint{GeneratedID: old.GeneratedID, ID: old.ID, RecordedAt: old.RecordedAt, Counts: old.Counts}
		for _, s := range old.Scenarios {
			hp.Duration = saturatingAdd(hp.Duration, s.Duration)
		}
		summary.History = append(summary.History, hp)
	}
	type observation struct{ samples, failures int }
	observations := map[string]observation{}
	for _, old := range history {
		for _, prev := range old.Scenarios {
			v := observations[prev.Identity]
			v.samples++
			if failedOutcome(prev.Status) {
				v.failures++
			}
			observations[prev.Identity] = v
		}
	}
	for i := range summary.Scenarios {
		s := &summary.Scenarios[i]
		v := observations[s.Identity]
		s.Samples = v.samples
		s.Failures = v.failures
		if len(history) > 0 {
			s.Samples++
			if failedOutcome(s.Status) {
				s.Failures++
			}
		}
	}
	for i := range summary.IssueRecords {
		issue := &summary.IssueRecords[i]
		if issue.State != statePassed {
			continue
		}
		issue.ConsecutiveUnobserved = 1
		for h := len(history) - 1; h >= 0; h-- {
			found := false
			for _, old := range history[h].IssueRecords {
				if old.Identity == issue.Identity && old.Ref == issue.Ref && old.State == statePassed {
					found = true
					break
				}
			}
			if !found {
				break
			}
			issue.ConsecutiveUnobserved++
		}
	}
	for _, s := range summary.Scenarios {
		if s.RerunUnavailable {
			summary.Diagnostics = append(summary.Diagnostics, Diagnostic{Path: "rerun", Message: "Selectors for redacted feature paths are omitted from rerun exports"})
			break
		}
	}
	summary.GateFailures = evaluateGates(summary, cfg.Gates)
	if len(summary.GateFailures) > 0 {
		redundant := summary.Computed.Tone == ToneFail
		for _, g := range summary.GateFailures {
			redundant = redundant && g == "unexpected failures"
		}
		if !redundant {
			summary.Computed = Verdict{Label: "Failed: quality gate (" + strings.Join(summary.GateFailures, "; ") + ")", Tone: ToneFail}
		}
	}
	for _, in := range inputs {
		summary.Diagnostics = append(summary.Diagnostics, in.Diagnostics...)
	}
	// Redact metadata, baseline and history exports as well as the current input.
	if cfg.Redaction != nil {
		if err = redactSummaries(&summary, allHistory, cfg.Redaction); err != nil {
			return nil, err
		}
	}
	for i := range allHistory {
		if runKey(allHistory[i]) == runKey(summary) {
			allHistory[i] = archiveSummary(summary)
		}
	}
	if archive.omitted {
		allHistory = nil
	}
	if err = checkPolicyFloor(summary, cfg.Gates); err != nil {
		return nil, err
	}
	classified.Analysis = &summary
	classified.Verdict = summary.Computed
	classified.VerdictSource = "computed"
	if err = checkScenarioOutcomes(features, classified, &policyCfg, original); err != nil {
		return nil, err
	}
	opt.Analysis = &summary
	opt.execution = metadata
	opt.BundleAttachments = opt.BundleAttachments || cfg.Bundle
	return &preparedRun{Features: features, Options: opt, Summary: summary, History: allHistory, Metadata: metadata, Bundle: cfg.Bundle, OmitHistory: archive.omitted}, nil
}

func validateMeta(m ScenarioMeta, sc input.Scenario) error {
	if err := validateInterval(m.Started, m.Finished); err != nil {
		return err
	}
	for i, a := range m.Attempts {
		if a.Number != i || !oneOf(a.Status, "passed", "failed", "undefined", "ambiguous", "pending", "skipped") {
			return invalid("attempt numbers must be contiguous from zero with valid statuses")
		}
		if err := validateInterval(a.Started, a.Finished); err != nil {
			return err
		}
		if len(a.Steps) > 0 {
			if err := input.Validate([]input.Feature{{Elements: []input.Scenario{{Steps: internalSteps(a.Steps, 0)}}}}); err != nil {
				return err
			}
			tmp := input.Scenario{Type: sc.Type, Steps: internalSteps(a.Steps, 0)}
			if string(tmp.Status()) != a.Status {
				return invalid("attempt status differs from evidence")
			}
		}
	}
	if len(m.Attempts) > 0 && m.Attempts[len(m.Attempts)-1].Status != string(modelmeta.ExecutionStatus(sc)) {
		return invalidInput("final attempt differs from scenario at line %d; preserve execution input order", sc.Line)
	}
	return nil
}

func validateInterval(start, end string) error {
	if start == "" && end == "" {
		return nil
	}
	a, e1 := time.Parse(time.RFC3339Nano, start)
	b, e2 := time.Parse(time.RFC3339Nano, end)
	if e1 != nil || e2 != nil || b.Before(a) {
		return invalid("execution interval")
	}
	return nil
}

var (
	generatedRunID  = regexp.MustCompile(`^run-[A-Z2-7]{26}$`)
	generatedTestID = regexp.MustCompile(`^test-[0-9a-f]{32}$`)
)

func validateSummary(s RunSummary) error {
	if s.GeneratedID && !generatedRunID.MatchString(s.ID) {
		return invalidSummary("invalid generated run id")
	}
	if s.Version > SummaryVersion {
		return invalid("summary schema %d was written by a newer version; use a version supporting that schema (supported schema %d)", s.Version, SummaryVersion)
	}
	if s.Version != SummaryVersion {
		return invalid("summary schema %d is unsupported; regenerate baselines and history with this version (schema %d)", s.Version, SummaryVersion)
	}
	if s.ID == "" {
		return invalidSummary("missing summary id")
	}
	if s.RecordedAt != "" {
		if _, err := time.Parse(time.RFC3339Nano, s.RecordedAt); err != nil {
			return invalidSummary("summary recordedAt must be RFC3339")
		}
	}
	for key, value := range s.Counts {
		if (!oneOf(key, statusOrder...) && key != "flaky") || value < 0 {
			return invalidSummary("summary counts must use supported statuses with nonnegative values")
		}
	}
	for _, issue := range s.IssueRecords {
		if issue.GeneratedIdentity && !generatedTestID.MatchString(issue.Identity) {
			return invalidSummary("invalid generated issue identity")
		}
		if !oneOf(issue.State, stateApplied, statePassed, stateNotApplied, stateInvalid, stateOutOfScope) {
			return invalidSummary("invalid summary issue state")
		}
	}
	seen := map[string]bool{}
	for _, sc := range s.Scenarios {
		if sc.GeneratedIdentity && !generatedTestID.MatchString(sc.Identity) {
			return invalidSummary("invalid generated scenario identity")
		}
		if sc.Identity == "" || seen[sc.Identity] || !oneOf(sc.Status, statusOrder...) || sc.Duration < 0 {
			return invalidSummary("invalid summary scenario")
		}
		seen[sc.Identity] = true
	}
	return nil
}

func compareRun(s *RunSummary, b RunSummary, gates GatePolicy) {
	c := &Comparison{Baseline: b.ID, NewFailures: []string{}, Fixed: []string{}, Missing: []string{}, Added: []string{}, NewlySkipped: []string{}, Slower: []string{}}
	old := map[string]ScenarioSummary{}
	for _, x := range b.Scenarios {
		old[x.Identity] = x
	}
	for i := range s.Scenarios {
		x := &s.Scenarios[i]
		prev, ok := old[x.Identity]
		if !ok {
			c.Added = append(c.Added, x.ID)
			x.Change = "added"
			if failedOutcome(x.Status) {
				c.NewFailures = append(c.NewFailures, x.ID)
				x.Change = "new-failure"
			}
			continue
		}
		delete(old, x.Identity)
		switch {
		case failedOutcome(x.Status) && !failedOutcome(prev.Status):
			c.NewFailures = append(c.NewFailures, x.ID)
			x.Change = "new-failure"
		case !failedOutcome(x.Status) && failedOutcome(prev.Status) && (x.Status == "passed" || x.Status == "known"):
			c.Fixed = append(c.Fixed, x.ID)
			x.Change = "fixed"
		case x.Status == "skipped" && prev.Status != "skipped":
			c.NewlySkipped = append(c.NewlySkipped, x.ID)
			x.Change = "newly-skipped"
		}
		x.DurationDelta = x.Duration - prev.Duration
		x.DurationComparable = x.Status == "passed" && prev.Status == "passed" && prev.Duration > 0
		if x.DurationComparable && x.DurationDelta > 0 && meetsRegressionFloor(x.DurationDelta, gates) {
			c.Slower = append(c.Slower, x.ID)
		}
	}
	for _, prev := range b.Scenarios {
		if _, ok := old[prev.Identity]; ok {
			c.Missing = append(c.Missing, prev.Identity)
		}
	}
	s.Comparison = c
}

func evaluateGates(s RunSummary, g GatePolicy) []string {
	var failures []string
	if g.Flaky && s.Counts["flaky"] > 0 {
		failures = append(failures, "flaky scenarios")
	}
	if g.Empty && s.ObservedScenarios == 0 {
		failures = append(failures, "no scenarios executed")
	}
	hookFailure := false
	for _, sc := range s.Scenarios {
		hookFailure = hookFailure || (sc.RunHook && failedOutcome(sc.Status))
	}
	if g.Unexpected && (s.Counts["failed"] > 0 || s.RunnerFailed || hookFailure) {
		failures = append(failures, "unexpected failures")
	}
	if g.Pending && s.Counts["pending"] > 0 {
		failures = append(failures, "pending scenarios")
	}
	if g.Undefined && (s.Counts["undefined"]+s.Counts["ambiguous"] > 0) {
		failures = append(failures, "undefined or ambiguous steps")
	}
	if g.Incomplete && s.Completeness != "complete" {
		failures = append(failures, "execution completeness not established")
	}
	if (g.NewFailures || g.MaxRegressionPercent > 0) && s.Comparison == nil && !g.AllowFirstRun {
		failures = append(failures, "baseline required")
	}
	if s.Comparison != nil {
		if g.NewFailures && len(s.Comparison.NewFailures) > 0 {
			failures = append(failures, "new failures")
		}
		if g.MaxRegressionPercent > 0 {
			for _, x := range s.Scenarios {
				base := x.Duration - x.DurationDelta
				if x.DurationComparable && durationRegressed(x.DurationDelta, base, g) {
					failures = append(failures, "duration regression")
					break
				}
			}
		}
	}
	return failures
}

func fingerprint(sc input.Scenario, p FingerprintPolicy) (string, string) {
	for _, st := range sc.Steps {
		if st.Result.Error == "" || st.Result.Status == input.StatusPassed {
			continue
		}
		lines := strings.Split(clean(st.Result.Error), "\n")
		n := min(len(lines), 1+p.StackLines)
		key := strings.Join(lines[:n], "\n")
		reason := "Error message"
		if p.StackLines > 0 {
			reason += " and stack prefix"
		}
		if p.IncludeStep {
			key = clean(st.Name) + "\n" + key
			reason += " with failing step"
		}
		for _, pattern := range p.Patterns {
			r, _ := regexp.Compile(pattern)
			key = r.ReplaceAllString(key, "<masked>")
		}
		if len(p.Patterns) > 0 {
			reason += "; configured patterns masked"
		}
		return key, reason
	}
	return "", ""
}

func applyIssuePolicies(features []input.Feature, ids [][]string, opt *renderOptions, cfg RunConfig) ([]Diagnostic, error) {
	var diagnostics []Diagnostic
	for _, p := range cfg.Issues {
		var re *regexp.Regexp
		var err error
		if p.ErrorPattern != "" {
			re, err = regexp.Compile(p.ErrorPattern)
			if err != nil {
				return nil, invalid("issue error pattern")
			}
		}
		for _, date := range []string{p.Expires, p.ReviewDate} {
			if date != "" {
				if _, err = time.Parse("2006-01-02", date); err != nil {
					return nil, invalid("issue date")
				}
				if cfg.RecordedAt == "" {
					return nil, invalid("dated issue policies need recordedAt")
				}
			}
		}
		for i := range opt.Annotations {
			a := &opt.Annotations[i]
			if a.Kind != KindKnown || a.Ref != p.Ref {
				continue
			}
			a.Owner = p.Owner
			a.Expires = p.Expires
			a.ReviewDate = p.ReviewDate
			reason := ""
			if len(p.Environments) > 0 && !slices.Contains(p.Environments, cfg.Environment) {
				reason = "Issue does not apply to this environment"
			}
			if p.Expires != "" && cfg.RecordedAt[:10] > p.Expires {
				reason = "Known-issue exception expired"
			}
			if re != nil {
				for fi, f := range features {
					for si, sc := range f.Elements {
						if ids[fi][si] != a.Scenario {
							continue
						}
						pos := annotationSourcePosition(sc, *a)
						if pos > 0 && pos <= len(sc.Steps) && !re.MatchString(sc.Steps[pos-1].Result.Error) {
							reason = "Failure does not match the expected issue signature"
						}
					}
				}
			}
			if reason != "" {
				a.PolicyReason = reason
				diagnostics = append(diagnostics, Diagnostic{Path: fmt.Sprintf("annotations[%d]", i), Message: reason})
			}
		}
	}
	for _, p := range cfg.Phases {
		if err := validateInterval(p.Started, p.Finished); err != nil {
			return nil, err
		}
		if p.Started == "" {
			return nil, invalid("phase timestamps required")
		}
	}
	return diagnostics, nil
}

// ErrGateFailed denotes a successfully written report whose explicit quality policy failed.
var ErrGateFailed = errors.New("report: quality gate failed")

// scenarioIdentity uses names for plain scenarios and source rows for outlines.
// Only genuine run hooks retain the synthetic source-line/name identity.
func scenarioIdentity(uri string, sc input.Scenario) string {
	if sc.Type == "run_hook" {
		return Identity(uri, fmt.Sprintf("row:%d\x00%s", sc.Line, sc.Name))
	}
	if modelmeta.ScenarioMetadata(&sc).IdentityNameCount > 1 {
		return Identity(uri, fmt.Sprintf("%s\x00row\x00%d", sc.Name, sc.Line))
	}
	return Identity(uri, sc.Name)
}

// decimalRat is the exact decimal the caller wrote (16.7 is 167/10, not the
// nearest binary float), so boundary comparisons do not depend on rounding.
func decimalRat(f float64) *big.Rat {
	r, ok := new(big.Rat).SetString(strconv.FormatFloat(f, 'g', -1, 64))
	if !ok {
		r = new(big.Rat).SetFloat64(f)
	}
	return r
}

func meetsRegressionFloor(delta int64, g GatePolicy) bool {
	if g.MinRegressionMs == nil {
		return delta >= int64(100*time.Millisecond)
	}
	floor := new(big.Rat).Mul(decimalRat(*g.MinRegressionMs), big.NewRat(1e6, 1))
	return new(big.Rat).SetInt64(delta).Cmp(floor) >= 0
}

// durationRegressed is the one predicate shared by the duration gate and the
// policy floor: delta*100 > pct*base in exact rational arithmetic.
func durationRegressed(delta, base int64, g GatePolicy) bool {
	if g.MaxRegressionPercent <= 0 || base <= 0 || delta <= 0 || !meetsRegressionFloor(delta, g) {
		return false
	}
	lhs := new(big.Rat).SetInt64(delta)
	lhs.Mul(lhs, big.NewRat(100, 1))
	rhs := new(big.Rat).SetInt64(base)
	rhs.Mul(rhs, decimalRat(g.MaxRegressionPercent))
	return lhs.Cmp(rhs) > 0
}

func invalidSummary(message string) error {
	return invalid("invalid summary: %s", message)
}

func uniqueTags(feature, scenario []input.Tag) []string {
	seen := map[string]bool{}
	var out []string
	for _, tags := range [][]input.Tag{feature, scenario} {
		for _, tag := range tags {
			if !seen[tag.Name] {
				seen[tag.Name] = true
				out = append(out, tag.Name)
			}
		}
	}
	return out
}

// declaredStepNames maps each known-issue annotation that declares an existing
// step to that step's display text, using the same position rules as the model.
func declaredStepNames(features []input.Feature, ids [][]string, annotations []Annotation) map[int]string {
	scenarios := map[string]input.Scenario{}
	for fi, f := range features {
		for si, sc := range f.Elements {
			scenarios[ids[fi][si]] = sc
		}
	}
	out := map[int]string{}
	for i, a := range annotations {
		sc, ok := scenarios[a.Scenario]
		if a.Kind != KindKnown || !ok || (a.Step == 0 && a.BackgroundStep == 0) {
			continue
		}
		position := a.Step
		if enhancedSteps(sc) {
			position = annotationSourcePosition(sc, a)
		}
		if position >= 1 && position <= len(sc.Steps) {
			out[i] = clean(sc.Steps[position-1].Name)
		}
	}
	return out
}

// originalEvidence is the unmasked step evidence and annotation set, keyed by the
// final scenario ID, that the floor uses for semantic checks after redaction.
type originalEvidence struct {
	scenarios   map[string]input.Scenario
	annotations []Annotation
}

func snapshotEvidence(features []input.Feature, ids [][]string, annotations []Annotation) (*originalEvidence, error) {
	clone, err := modelmeta.CloneFeatures(features)
	if err != nil {
		return nil, err
	}
	o := &originalEvidence{scenarios: map[string]input.Scenario{}, annotations: slices.Clone(annotations)}
	for fi, f := range clone {
		for si, sc := range f.Elements {
			o.scenarios[ids[fi][si]] = sc
		}
	}
	return o, nil
}

// remap rekeys the snapshot from pre-redaction to final navigation IDs.
func (o *originalEvidence) remap(navigation map[string]string) {
	if o == nil {
		return
	}
	scenarios := make(map[string]input.Scenario, len(o.scenarios))
	for id, sc := range o.scenarios {
		if final, ok := navigation[id]; ok {
			id = final
		}
		scenarios[id] = sc
	}
	o.scenarios = scenarios
	for i := range o.annotations {
		if final, ok := navigation[o.annotations[i].Scenario]; ok {
			o.annotations[i].Scenario = final
		}
	}
}

// retryStepPair names an earlier-attempt step and a final-attempt step at the
// same execution position whose keyword or name differ before redaction.
type retryStepPair struct {
	feature, scenario, current, attempt, step int
}

// differingRetrySteps lists, using the positions the model matches retries by
// (origin, own index and background ordinal), the step pairs that differ.
func differingRetrySteps(features []input.Feature, ids [][]string, metadata map[string]ScenarioMeta) []retryStepPair {
	position := func(steps []input.Step) []string {
		out := make([]string, len(steps))
		background := 0
		for i := range steps {
			st := modelmeta.StepMetadata(&steps[i])
			ordinal := 0
			if st.Origin == "background" {
				background++
				ordinal = background
			}
			out[i] = fmt.Sprintf("%s/%d/%d", st.Origin, st.OwnIndex, ordinal)
		}
		return out
	}
	var out []retryStepPair
	for fi, f := range features {
		for si, sc := range f.Elements {
			attempts := metadata[ids[fi][si]].Attempts
			if len(attempts) < 2 || len(modelmeta.ScenarioMetadata(&sc).PreviousAttempts) == 0 {
				continue
			}
			// Hooks are never known-issue steps, so only declared steps are paired.
			current := map[string][]int{}
			for c, key := range position(sc.Steps) {
				if modelmeta.StepMetadata(&sc.Steps[c]).Origin != "hook" {
					current[key] = append(current[key], c)
				}
			}
			for a, attempt := range attempts[:len(attempts)-1] {
				steps := internalSteps(attempt.Steps, 0)
				for k, key := range position(steps) {
					for _, c := range current[key] {
						if steps[k].Name != sc.Steps[c].Name || steps[k].Keyword != sc.Steps[c].Keyword {
							out = append(out, retryStepPair{fi, si, c, a, k})
						}
					}
				}
			}
		}
	}
	return out
}
