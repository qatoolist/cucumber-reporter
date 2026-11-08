package evidence

import "testing"

func TestOutcomeOracleCatchesDisabledRunnerReconciliation(t *testing.T) {
	for _, status := range []string{"pending", "undefined", "unknown"} {
		raw := Stream{Failed: true, Cases: map[string][]Execution{"known": {{Results: []string{"failed"}}}, "other": {{Results: []string{status}}}}}

		if err := CheckRunner(RunnerAudit{Failed: true, Cases: [][]Execution{raw.Cases["known"], raw.Cases["other"]}}.Encode(), false); err == nil {
			t.Fatalf("unreported runner failure accepted with %s case", status)
		}
	}
}

func TestOutcomeOracleCatchesDisabledRetryRule(t *testing.T) {
	for _, finalStatus := range []string{"pending", "skipped"} {
		attempts := []Execution{{Results: []string{"failed"}}, {Results: []string{finalStatus}}}
		raw := Stream{Cases: map[string][]Execution{"case": attempts}}
		for _, incorrectlyClaimed := range []string{"passed", finalStatus} {
			if err := CheckOutcomes([]Stream{raw}, map[string]Final{"case": {Status: incorrectlyClaimed, Attempts: attempts}}, false); err == nil {
				t.Fatalf("accepted %s after failed->%s", incorrectlyClaimed, finalStatus)
			}
		}
		if err := CheckOutcomes([]Stream{raw}, map[string]Final{"case": {Status: "failed", Attempts: attempts}}, true); err != nil {
			t.Fatal(err)
		}
	}
}

func TestOutcomeOracleEmptyCaseCannotExplainRunnerFailure(t *testing.T) {
	raw := Stream{Failed: true, Cases: map[string][]Execution{"known": {{Results: []string{"failed"}}}, "empty": {{Results: []string{"passed"}, NoOwnSteps: true}}}}

	if err := CheckRunner(RunnerAudit{Failed: true, Cases: [][]Execution{raw.Cases["known"], raw.Cases["empty"]}}.Encode(), false); err == nil {
		t.Fatal("empty scenario with passing hooks hid runner failure")
	}
}

func TestRawAuditDoesNotUseParsedFailureFlags(t *testing.T) {
	var raw Messages
	for _, event := range []string{
		`{"testCaseStarted":{"id":"one","testCaseId":"flaky"}}`,
		`{"testStepFinished":{"testCaseStartedId":"one","testStepResult":{"status":"FAILED"}}}`,
		`{"testCaseStarted":{"id":"two","testCaseId":"flaky"}}`,
		`{"testStepFinished":{"testCaseStartedId":"two","testStepResult":{"status":"PASSED"}}}`,
		`{"testCaseStarted":{"id":"three","testCaseId":"known"}}`,
		`{"testStepFinished":{"testCaseStartedId":"three","testStepResult":{"status":"FAILED"}}}`,
		`{"testRunFinished":{"success":false}}`,
	} {
		if err := raw.Observe([]byte(event)); err != nil {
			t.Fatal(err)
		}
	}
	if err := CheckRunner(raw.Audit().Encode(), false); err == nil {
		t.Fatal("raw strict-flaky signal vanished")
	}
	if err := CheckRunner(raw.Audit().Encode(), true); err != nil {
		t.Fatal(err)
	}
}

func TestRawLegacyAuditRejectsGreenerClaim(t *testing.T) {
	raw := []byte(`[{"uri":"a.feature","elements":[{"name":"same","line":3,"steps":[{"result":{"status":"failed"}}]},{"name":"same","line":3,"steps":[{"result":{"status":"pending"}}]}]}]`)
	audit, err := LegacyAudit(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckCases(audit, map[string]Final{LocationKey("a.feature", 3, "same"): {Status: "passed"}}); err == nil {
		t.Fatal("raw failed/pending became passed")
	}
}
