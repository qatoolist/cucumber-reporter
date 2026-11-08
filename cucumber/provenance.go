package cucumber

// Provenance records parser context explicitly so equal copies behave equally.
type (
	// FeatureProvenance holds navigation and full-document identity context.
	FeatureProvenance struct {
		OrphanBackground  bool
		BackgroundSlots   map[string]string
		IdentityCatalog   map[string]int `json:",omitempty"`
		ReportID          string         `json:",omitempty"`
		SyntheticRunHooks bool           `json:",omitempty"`
	}
	// ScenarioProvenance holds parser-derived collision and Background context.
	ScenarioProvenance struct {
		// RawCaseAudit preserves per-scenario status observations from input bytes. It is an
		// opaque, parser-owned encoding: callers must copy it unchanged and must not set or edit it.
		// The report checks it against the scenario's results; an empty value means the scenario has
		// no raw artifact (a caller-constructed model) and that check is skipped.
		RawCaseAudit string `json:",omitempty"`
		// RawRunnerAudit preserves an independent, status-only scan of input envelopes. It is opaque
		// and parser-owned like RawCaseAudit; an empty value skips the runner-signal check.
		RawRunnerAudit       string `json:",omitempty"`
		RunnerFailed         bool
		InferredRetryFailure bool
		CaseAttachments      bool
		ReportedAttempts     []Status
		MissingStepResults   bool
		OutcomeFloor         []ExecutionEvidence
		AttemptBackground    []Step
		PreviousAttempts     []Scenario
		EnhancedFolded       bool
		PlainFolded          bool
		NavigationSuffix     string
		IdentityNameCount    int  `json:",omitempty"`
		BackgroundUnresolved bool `json:",omitempty"`
	}
	// StepProvenance holds the execution origin and declared own-step index.
	StepProvenance struct {
		HookKind string
		HookStep int
		Origin   string `json:",omitempty"`
		OwnIndex int    `json:",omitempty"`
	}
)

// ExecutionEvidence preserves the result-only floor captured before normalization.
// Callers constructing models may leave provenance unset; parsed provenance should
// be copied with its enclosing value and not synthesized from untrusted options.
type ExecutionEvidence struct {
	Results []Status
	Own     bool
}
