package evidence

import "testing"

func TestWorstDuplicateAndMissingResult(t *testing.T) {
	raw, err := Scan([]byte(`{"result":{"status":"failed","ſtatus":"passed"}}`))
	if err != nil || raw.Results["failed"] != 1 {
		t.Fatal(raw, err)
	}
	passed, _ := Scan([]byte(`{"result":{"status":"passed"}}`))
	if Check(raw, passed) == nil {
		t.Fatal("lost failure accepted")
	}
}

func TestRunnerFailureSignal(t *testing.T) {
	got, err := Scan([]byte(`{"testRunFinished":{"success":false}}`))
	if err != nil || !got.RunnerFailed {
		t.Fatalf("failure signal lost: %+v %v", got, err)
	}
	got, err = Scan([]byte(`{"testRunFinished":{"success":false,"Success":true}}`))
	if err != nil || !got.RunnerFailed {
		t.Fatalf("later value erased runner failure: %+v %v", got, err)
	}
}
