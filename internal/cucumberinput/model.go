// Package cucumber parses and validates Cucumber execution results.
package cucumber

import modelmeta "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"

// Feature is the public execution model; renderer bookkeeping is private.
type (
	Feature = modelmeta.Feature
	// Scenario is the public execution model; renderer bookkeeping is private.
	Scenario = modelmeta.Scenario
	// Step is the public execution model; renderer bookkeeping is private.
	Step = modelmeta.Step
	// Tag is the public execution model; renderer bookkeeping is private.
	Tag = modelmeta.Tag
	// DocString is the public execution model; renderer bookkeeping is private.
	DocString = modelmeta.DocString
	// Row is the public execution model; renderer bookkeeping is private.
	Row = modelmeta.Row
	// Match is the public execution model; renderer bookkeeping is private.
	Match = modelmeta.Match
	// Embedding is the public execution model; renderer bookkeeping is private.
	Embedding = modelmeta.Embedding
	// Result is the public execution model; renderer bookkeeping is private.
	Result = modelmeta.Result
	// Limits is the public execution model; renderer bookkeeping is private.
	Limits = modelmeta.Limits
)

// DefaultInputLimit is the default parser or attachment limit.
const (
	DefaultInputLimit = modelmeta.DefaultInputLimit
	// DefaultEmbeddingLimit is the default parser or attachment limit.
	DefaultEmbeddingLimit = modelmeta.DefaultEmbeddingLimit
	// DefaultInlineAttachmentLimit is the default parser or attachment limit.
	DefaultInlineAttachmentLimit = modelmeta.DefaultInlineAttachmentLimit
)
