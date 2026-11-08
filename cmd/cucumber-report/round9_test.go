package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/qatoolist/cucumber-reporter/report"
)

func TestRound9ImportDiagnosticsCLI(t *testing.T) {
	input := newRun(t, []byte(`[{"uri":"case.feature","elements":[{"name":"case","line":1,"steps":[{"result":{"status":"passed"}}]}]}]`))
	for _, tc := range []struct{ flag, body, want string }{
		{"-baseline", `{"version":3,"oldSecretField":"SECRET"}`, "earlier version"},
		{"-history", `[{"version":3,"oldSecretField":"SECRET"}]`, "earlier version"},
		{"-run-config", `{"oldSecretField":"SECRET"}`, "invalid run configuration"},
		{"-baseline", fmt.Sprintf(`{"version":%d}`, report.SummaryVersion+1), "written by a newer version"},
		{"-history", fmt.Sprintf(`[{"version":%d}]`, report.SummaryVersion+1), "written by a newer version"},
		{"-baseline", fmt.Sprintf(`{"version":%d,"id":"old","counts":{"SECRET":1}}`, report.SummaryVersion), "counts"},
		{"-history", fmt.Sprintf(`[{"version":%d,"id":"old","counts":{"SECRET":1}}]`, report.SummaryVersion), "counts"},
		{"-run-config", `{"gates":{"minRegressionMs":-1}}`, "minRegressionMs must"},
	} {
		t.Run(tc.flag+"/"+tc.want, func(t *testing.T) {
			_, stderr, code := command(t, "-enhanced", tc.flag, writeFile(t, tc.body), input)
			if code != 1 || !strings.Contains(stderr, tc.want) || strings.Contains(stderr, "SECRET") {
				t.Fatalf("unhelpful error: %d %s", code, stderr)
			}
			if tc.flag != "-run-config" && !strings.Contains(stderr, tc.flag) {
				t.Fatal("import flag missing")
			}
		})
	}
}
