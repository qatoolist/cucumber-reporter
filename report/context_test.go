package report_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/qatoolist/cucumber-reporter/cucumber"
	"github.com/qatoolist/cucumber-reporter/report"
)

func TestCancelledRenderDoesNotWrite(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f := []cucumber.Feature{{Elements: []cucumber.Scenario{{Steps: []cucumber.Step{{Result: cucumber.Result{Status: cucumber.StatusPassed}}}}}}}
	var output bytes.Buffer
	if err := report.RenderToContext(ctx, &output, f, report.Options{Version: 2}); !errors.Is(err, context.Canceled) || output.Len() != 0 {
		t.Fatalf("canceled render wrote bytes: %v", err)
	}
	b, err := report.RenderContext(ctx, f, report.Options{Version: 2})
	if !errors.Is(err, context.Canceled) || b != nil {
		t.Fatal("canceled byte render returned output")
	}
}
