package cucumber

import (
	"time"

	modelmeta "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"
)

// FormatDuration formats a duration for display.
func FormatDuration(d time.Duration) string { return modelmeta.FormatDuration(d) }
