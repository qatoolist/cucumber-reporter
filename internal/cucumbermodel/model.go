// Package cucumbermodel owns the internal execution model and renderer metadata.
package cucumbermodel

import "time"

// Feature is one feature file's results.
type Feature struct {
	featureState                        FeatureState
	URI, ID, Keyword, Name, Description string
	Line                                int
	Tags                                []Tag
	Elements                            []Scenario
}

// Scenario is one executed scenario (one row of an outline is one scenario).
type Scenario struct {
	scenarioState ScenarioState
	// Before and After are legacy hook evidence retained by ParseEnhanced and folded by PrepareRun.
	Before, After                        []Step
	ID, Keyword, Name, Description, Type string
	Line                                 int
	Tags                                 []Tag
	Steps                                []Step
}

// Step is one executed step with its result, arguments and attachments.
type Step struct {
	stepState StepState
	// Before and After retain step-local hooks until enhanced execution folding.
	Before, After []Step `json:",omitempty"`
	Keyword, Name string
	Line          int
	DocString     *DocString
	Rows          []Row
	Match         Match
	Result        Result
	Embeddings    []Embedding
}

// Tag is a Gherkin tag such as "@smoke".
type Tag struct {
	Name string
	Line int
}

// DocString is a step's doc string argument.
type DocString struct {
	Value, ContentType string
	Line               int
}

// Row is one data table row.
type Row struct {
	Cells []string
}

// Match is the step definition location reported by the runner.
type Match struct {
	Location string
}

// Embedding is one attachment. Data is the base64 text exactly as it appeared in the input; use
// Decode to obtain the bytes.
type Embedding struct {
	Name, MimeType, Data string
}

// Result is a step outcome. Duration is nil when the input has none: Godog omits it for undefined,
// pending, skipped and ambiguous steps.
type Result struct {
	Status   Status
	Error    string
	Duration *time.Duration
}

// Limits bounds the work Parse and the renderer accept. A zero or negative field selects its default.
type Limits struct {
	// Input is the largest accepted cucumber.json, in bytes. Default 512 MiB.
	Input int64 `json:"input,omitempty"`
	// Embedding is the largest accepted base64 text of one attachment, in characters. Default 64 MiB.
	Embedding int64 `json:"embedding,omitempty"`
	// InlineAttachment is the largest decoded attachment shown inside the page, in bytes. Larger text
	// attachments are written beside the page by report.RenderDir, or left out by report.Render.
	// Default 4 MiB.
	InlineAttachment int64 `json:"inlineAttachment,omitempty"`
}

// Default limits.
const (
	DefaultInputLimit            int64 = 512 << 20
	DefaultEmbeddingLimit        int64 = 64 << 20
	DefaultInlineAttachmentLimit int64 = 4 << 20
)

// WithDefaults returns l with every unset field replaced by its default.
func (l Limits) WithDefaults() Limits {
	if l.Input <= 0 {
		l.Input = DefaultInputLimit
	}
	if l.Embedding <= 0 {
		l.Embedding = DefaultEmbeddingLimit
	}
	if l.InlineAttachment <= 0 {
		l.InlineAttachment = DefaultInlineAttachmentLimit
	}
	return l
}
