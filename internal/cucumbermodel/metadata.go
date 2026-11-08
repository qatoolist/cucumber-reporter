package cucumbermodel

// FeatureState and the other sidecars are private fields of the internal model.
// Their accessors are restricted to this module by the internal package boundary.
type (
	// FeatureState holds navigation and full-document identity context.
	FeatureState struct {
		OrphanBackground  bool
		BackgroundSlots   map[string]string
		IdentityCatalog   map[string]int `json:",omitempty"`
		ReportID          string         `json:",omitempty"`
		SyntheticRunHooks bool           `json:",omitempty"`
	}
	// ScenarioState holds parser-derived collision and Background context.
	ScenarioState struct {
		// RawCaseAudit preserves per-scenario status observations from input bytes.
		RawCaseAudit string `json:",omitempty"`
		// RawRunnerAudit preserves an independent, status-only scan of input envelopes.
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
	// StepState holds the execution origin and declared own-step index.
	StepState struct {
		HookKind string
		HookStep int
		Origin   string `json:",omitempty"`
		OwnIndex int    `json:",omitempty"`
	}
)

// FeatureMetadata returns renderer-owned feature state.
func FeatureMetadata(f *Feature) *FeatureState { return &f.featureState }

// ScenarioMetadata returns parser-owned scenario context.
func ScenarioMetadata(s *Scenario) *ScenarioState { return &s.scenarioState }

// StepMetadata returns execution-origin and declared-index state.
func StepMetadata(s *Step) *StepState { return &s.stepState }

// MessageAttempt preserves runner execution plus a private full-document name catalog.
type MessageAttempt struct {
	URI                             string
	FeatureName, FeatureDescription string
	FeatureTags                     []Tag
	Scenario                        Scenario
	Number                          int
	Started, Finished, Worker       string
	RunnerFailed                    bool
	catalog                         map[string]int
}

// AttemptCatalog returns the full-document name catalog.
func AttemptCatalog(a *MessageAttempt) map[string]int { return a.catalog }

// SetAttemptCatalog associates a parsed attempt with its full document.
func SetAttemptCatalog(a *MessageAttempt, c map[string]int) { a.catalog = c }

// MessageProvenance follows the shared step backing array when attempts are copied.
type MessageProvenance struct {
	Step     StepState
	Scenario ScenarioState
	Catalog  map[string]int
}
