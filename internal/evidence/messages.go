package evidence

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrMissingRunSuccess reports a testRunFinished envelope without its success field.
var ErrMissingRunSuccess = errors.New("testRunFinished.success is missing")

// MessageFieldError reports an envelope field whose JSON value has the wrong type.
type MessageFieldError struct{ Field, Want string }

func (e *MessageFieldError) Error() string {
	return fmt.Sprintf("Messages envelope field %s has the wrong type (want %s)", e.Field, e.Want)
}

// messageFieldName spells a decoder field path with Messages (camelCase) names.
func messageFieldName(field string) string {
	parts := strings.Split(field, ".")
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToLower(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, ".")
}

// Messages independently observes raw start/result/finish envelopes. It does not
// use parser cases, retry counters, willBeRetried, or normalized statuses.
type Messages struct {
	starts     map[string][]string
	results    map[string]Execution
	failed     bool
	hookFailed bool
	pickles    map[string]rawPickle
	cases      map[string]string
	lines      map[string]int
	background map[string]bool
	planned    map[string][]string
	observed   map[string]map[string]bool
}

type rawPickle struct {
	ID, URI, Name string
	AST           []string `json:"astNodeIds"`
	Steps         []struct {
		AST []string `json:"astNodeIds"`
	}
}

// Observe records raw execution evidence from one Messages envelope.
func (m *Messages) Observe(raw []byte) error {
	var event struct {
		Pickle *rawPickle `json:"pickle"`
		Case   *struct {
			ID, PickleID string
			TestSteps    []struct{ ID string }
		} `json:"testCase"`
		Document any                              `json:"gherkinDocument"`
		Start    *struct{ ID, TestCaseID string } `json:"testCaseStarted"`
		Result   *struct {
			TestCaseStartedID string
			TestStepID        string
			TestStepResult    struct{ Status string }
		} `json:"testStepFinished"`
		RunHook *struct {
			Result         struct{ Status string }
			TestStepResult struct{ Status string }
		} `json:"testRunHookFinished"`
		Finish *struct{ Success *bool } `json:"testRunFinished"`
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) && typeErr.Field != "" {
			return &MessageFieldError{Field: messageFieldName(typeErr.Field), Want: strings.TrimPrefix(typeErr.Type.String(), "*")}
		}
		return err
	}
	if m.starts == nil {
		m.pickles = map[string]rawPickle{}
		m.cases = map[string]string{}
		m.lines = map[string]int{}
		m.background = map[string]bool{}
		m.planned = map[string][]string{}
		m.observed = map[string]map[string]bool{}
		m.starts = map[string][]string{}
		m.results = map[string]Execution{}
	}
	if event.Pickle != nil {
		m.pickles[event.Pickle.ID] = *event.Pickle
	}
	if event.Case != nil {
		m.cases[event.Case.ID] = event.Case.PickleID
		for _, st := range event.Case.TestSteps {
			m.planned[event.Case.ID] = append(m.planned[event.Case.ID], st.ID)
		}
	}
	var walk func(any)
	walk = func(v any) {
		switch v := v.(type) {
		case map[string]any:
			if id, ok := v["id"].(string); ok {
				if loc, ok := v["location"].(map[string]any); ok {
					if line, ok := loc["line"].(float64); ok {
						m.lines[id] = int(line)
					}
				}
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		}
	}
	if event.Document != nil {
		walk(event.Document)
		var backgrounds func(any, bool)
		backgrounds = func(v any, in bool) {
			switch v := v.(type) {
			case map[string]any:
				if in {
					if id, ok := v["id"].(string); ok {
						m.background[id] = true
					}
				}
				for key, child := range v {
					backgrounds(child, in || key == "background")
				}
			case []any:
				for _, child := range v {
					backgrounds(child, in)
				}
			}
		}
		backgrounds(event.Document, false)
	}
	if event.Start != nil {
		s := event.Start
		m.starts[s.TestCaseID] = append(m.starts[s.TestCaseID], s.ID)
	}
	if event.Result != nil {
		s := event.Result
		if m.observed[s.TestCaseStartedID] == nil {
			m.observed[s.TestCaseStartedID] = map[string]bool{}
		}
		m.observed[s.TestCaseStartedID][s.TestStepID] = true
		e := m.results[s.TestCaseStartedID]
		e.Results = append(e.Results, s.TestStepResult.Status)
		m.results[s.TestCaseStartedID] = e
	}
	if event.RunHook != nil {
		for _, status := range []string{event.RunHook.Result.Status, event.RunHook.TestStepResult.Status} {
			if status == "FAILED" || status == "AMBIGUOUS" {
				m.hookFailed = true
			}
		}
	}
	if event.Finish != nil {
		if event.Finish.Success == nil {
			return ErrMissingRunSuccess
		}
		if !*event.Finish.Success {
			m.failed = true
		}
	}
	return nil
}

// Check requires a visible non-passing result on every retried execution, even
// when the runner omitted the Around hook event that caused its retry.
func (m *Messages) Check(shown map[string]Execution, runnerFailed bool) error {
	if m.failed && !runnerFailed {
		return Floorf("evidence floor: raw runner failure signal disappeared; this is a reporter bug")
	}
	if m.failed && len(shown) == 0 {
		return Floorf("evidence floor: runner failed without executed cases; this is a reporter bug")
	}
	for _, ids := range m.starts {
		for i, id := range ids {
			e, ok := shown[id]
			if !ok {
				return Floorf("evidence floor: raw testCaseStarted has no execution; this is a reporter bug")
			}
			if i < len(ids)-1 && strictlyPassed(e) {
				return Floorf("evidence floor: retried execution has no non-passing evidence; this is a reporter bug")
			}
			raw := Counts{Results: map[string]int{}}
			actual := Counts{Results: map[string]int{}}
			for _, s := range m.results[id].Results {
				raw.Add(s)
			}
			for _, s := range e.Results {
				actual.Add(s)
			}
			if err := Check(raw, actual); err != nil {
				return err
			}
		}
	}
	return nil
}
