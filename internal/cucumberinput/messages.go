package cucumber

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/qatoolist/cucumber-reporter/internal/evidence"

	modelmeta "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"
)

// MessageAttempt is one Cucumber Messages test-case execution, including hook steps.
type MessageAttempt = modelmeta.MessageAttempt

// MessageRun retains every attempt; consumers group by URI and scenario identity.
type MessageRun struct {
	Attempts []MessageAttempt
	// Started and Finished are measured run timestamps in RFC3339Nano.
	Started, Finished string
	// Git fields are reported by the runner, not discovered from the local checkout.
	Revision, Branch, Remote string
}

type messageTime struct {
	Seconds json.Number `json:"seconds"`
	Nanos   int64       `json:"nanos"`
}

func (t messageTime) string() (string, error) {
	s, err := strconv.ParseInt(string(t.Seconds), 10, 64)
	if err != nil || t.Nanos < 0 || t.Nanos >= 1e9 || s < -62135596800 || s > 253402300799 {
		return "", ErrInvalid
	}
	return time.Unix(s, t.Nanos).UTC().Format(time.RFC3339Nano), nil
}

type messageDuration struct {
	Seconds json.Number `json:"seconds"`
	Nanos   int64       `json:"nanos"`
}

func (d messageDuration) duration() (time.Duration, error) {
	s, err := strconv.ParseInt(string(d.Seconds), 10, 64)
	if err != nil || s < 0 || d.Nanos < 0 || d.Nanos >= 1e9 || s > (math.MaxInt64-d.Nanos)/1e9 {
		return 0, ErrInvalid
	}
	return time.Duration(s*1e9 + d.Nanos), nil
}

type pickleStep struct {
	AstNodeIDs []string
	ID, Text   string
	Argument   struct {
		DocString *struct{ Content, MediaType string }
		DataTable *struct {
			Rows []struct{ Cells []struct{ Value string } }
		}
	}
}
type messagePickle struct {
	ID, URI, Name string
	AstNodeIDs    []string
	Tags          []struct{ Name string }
	Steps         []pickleStep
}
type messageCase struct {
	ID, PickleID string
	TestSteps    []struct {
		ID, PickleStepID, HookID string
		StepDefinitionIDs        []string
	}
}
type messageStart struct {
	ID, TestCaseID, WorkerID string
	Attempt                  int
	Timestamp                messageTime
}
type messageFinish struct {
	TestCaseStartedID string
	WillBeRetried     bool
	Timestamp         messageTime
}
type messageStepResult struct {
	Status, Message string
	Duration        messageDuration
	Exception       *struct{ Type, Message, StackTrace string }
}

func (r messageStepResult) errorText() string {
	if r.Message != "" {
		return r.Message
	}
	if r.Exception != nil {
		return strings.Join([]string{r.Exception.Type, r.Exception.Message, r.Exception.StackTrace}, "\n")
	}
	return ""
}

type (
	messageResult struct {
		TestCaseStartedID, TestStepID string
		TestStepResult                messageStepResult
	}
	messageAttachment struct{ TestCaseStartedID, TestStepID, TestRunHookStartedID, Body, MediaType, ContentEncoding, FileName string }
	runHookStart      struct {
		ID, HookID, WorkerID string
		Timestamp            messageTime
	}
	runHookFinish struct {
		TestRunHookStartedID string
		Timestamp            messageTime
		Result               messageStepResult
	}
)

// ParseMessages reads bounded NDJSON envelopes, preserving retries, hooks and attachments.
// Informational envelopes are ignored. External URLs are refused rather than fetched.
// Run hooks become explicitly labeled synthetic scenarios so their failures remain visible.
func ParseMessages(r io.Reader, limits Limits) (MessageRun, error) {
	lim := limits.WithDefaults()
	utf8, encErr := utf8Reader(r)
	if encErr != nil {
		return MessageRun{}, encErr
	}
	reader := &io.LimitedReader{R: utf8, N: lim.Input + 1}
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64<<10), int(min(lim.Input+1, int64(math.MaxInt))))
	pickles := map[string]messagePickle{}
	cases := map[string]messageCase{}
	starts := map[string]messageStart{}
	finishes := map[string]messageFinish{}
	results := map[string]messageResult{}
	attachments := map[string][]messageAttachment{}
	lines := map[string]int{}
	keywords := map[string]string{}
	backgroundSteps := map[string]bool{}
	featureInfo := map[string]Feature{}
	nameCatalog := map[string]map[string]int{}
	hasBackground := map[string]bool{}
	hooks := map[string]string{}
	hookKinds := map[string]string{}
	definitions := map[string]string{}
	order := []string{}
	line := 0
	var rawFloor evidence.Counts
	var rawOracle evidence.Messages
	oracleShown := map[string]evidence.Execution{}
	caseFloors := map[string]evidence.Counts{}
	runFinished := false
	runFailed := false
	runHooks := map[string]runHookStart{}
	runHookResults := map[string]runHookFinish{}
	var hookOrder []string
	var out MessageRun
	decode := func(raw json.RawMessage, v any) error {
		d := json.NewDecoder(bytes.NewReader(raw))
		d.UseNumber()
		if err := d.Decode(v); err != nil {
			return ErrInvalidJSON
		}
		return nil
	}
	for scanner.Scan() {
		line++
		raw := scanner.Bytes()
		if len(bytes.TrimSpace(raw)) == 0 {
			continue
		}
		if e := rawOracle.Observe(raw); e != nil {
			if errors.Is(e, evidence.ErrMissingRunSuccess) {
				return MessageRun{}, atPath(fmt.Errorf("%w: %w", ErrInvalid, e), "message[%d]", line)
			}
			var fieldErr *evidence.MessageFieldError
			if errors.As(e, &fieldErr) {
				return MessageRun{}, atPath(fmt.Errorf("%w: %w", ErrInvalid, fieldErr), "message[%d]", line)
			}
			return MessageRun{}, atPath(ErrInvalidJSON, "message[%d]", line)
		}
		ledger, scanErr := evidence.Scan(raw)
		if scanErr != nil {
			return MessageRun{}, atPath(ErrInvalidJSON, "message[%d]", line)
		}
		rawFloor.RunnerFailed = rawFloor.RunnerFailed || ledger.RunnerFailed
		for status, n := range ledger.Results {
			for j := 0; j < n; j++ {
				rawFloor.Add(status)
			}
		}
		// Reuse duplicate-key and depth validation on a one-element array.
		framed := append([]byte{'['}, raw...)
		framed = append(framed, ']')
		if err := checkMessageSyntax(framed); err != nil {
			return MessageRun{}, atPath(ErrInvalidJSON, "message[%d]", line)
		}
		var env map[string]json.RawMessage
		if err := decode(raw, &env); err != nil || len(env) != 1 {
			return MessageRun{}, atPath(ErrInvalidJSON, "message[%d]", line)
		}
		for kind, b := range env {
			var err error
			switch kind {
			case "TestStepFinished":
				err = fmt.Errorf("%w: Messages envelope names are case-sensitive; expected testStepFinished", ErrInvalid)
			case "pickle":
				var v messagePickle
				err = decode(b, &v)
				if v.ID == "" || pickles[v.ID].ID != "" {
					err = ErrInvalid
				}
				pickles[v.ID] = v
			case "testCase":
				var v messageCase
				err = decode(b, &v)
				if v.ID == "" || cases[v.ID].ID != "" {
					err = ErrInvalid
				}
				cases[v.ID] = v
			case "testCaseStarted":
				var v messageStart
				err = decode(b, &v)
				if v.ID == "" || starts[v.ID].ID != "" || v.Attempt < 0 {
					err = ErrInvalid
				}
				starts[v.ID] = v
				order = append(order, v.ID)
			case "testCaseFinished":
				var v messageFinish
				err = decode(b, &v)
				if v.TestCaseStartedID == "" || finishes[v.TestCaseStartedID].TestCaseStartedID != "" {
					err = ErrInvalid
				}
				finishes[v.TestCaseStartedID] = v
			case "testStepFinished":
				var v messageResult
				err = decode(b, &v)
				key := v.TestCaseStartedID + "\x00" + v.TestStepID
				if results[key].TestStepID != "" {
					err = ErrInvalid
				}
				results[key] = v
				scoped := caseFloors[v.TestCaseStartedID]
				for status, n := range ledger.Results {
					for j := 0; j < n; j++ {
						scoped.Add(status)
					}
				}
				caseFloors[v.TestCaseStartedID] = scoped
			case "attachment":
				var v messageAttachment
				err = decode(b, &v)
				if v.TestCaseStartedID == "" && v.TestRunHookStartedID == "" {
					err = ErrIncomplete
				}
				key := v.TestCaseStartedID + "\x00" + v.TestStepID
				if v.TestRunHookStartedID != "" {
					key = "run-hook\x00" + v.TestRunHookStartedID
				}
				attachments[key] = append(attachments[key], v)
			case "gherkinDocument":
				var v any
				err = decode(b, &v)
				collectMessageLines(v, lines)
				collectStepKeywords(v, keywords)
				collectBackgroundSteps(v, backgroundSteps)
				var g struct {
					URI     string
					Feature struct {
						Name, Description string
						Tags              []Tag
					}
				}
				if e := decode(b, &g); e != nil {
					err = e
				}
				catalog := map[string]int{}
				collectScenarioNames(v, catalog)
				nameCatalog[g.URI] = catalog
				hasBackground[g.URI] = documentHasBackground(v)
				featureInfo[g.URI] = Feature{Name: g.Feature.Name, Description: g.Feature.Description, Tags: g.Feature.Tags}
			case "hook":
				var v struct{ ID, Name, Type string }
				err = decode(b, &v)
				hooks[v.ID] = v.Name
				hookKinds[v.ID] = strings.ToLower(v.Type)
				if v.Name == "" && v.Type != "" {
					hooks[v.ID] = strings.ReplaceAll(strings.ToLower(v.Type), "_", " ") + " hook"
				}
			case "stepDefinition":
				var v struct {
					ID              string
					SourceReference struct {
						URI      string
						Location struct{ Line int }
					}
				}
				err = decode(b, &v)
				definitions[v.ID] = fmt.Sprintf("%s:%d", v.SourceReference.URI, v.SourceReference.Location.Line)
			case "meta":
				var v struct {
					CI *struct {
						Git *struct{ Revision, Branch, Remote string }
					}
				}
				err = decode(b, &v)
				if v.CI != nil && v.CI.Git != nil {
					out.Revision, out.Branch, out.Remote = v.CI.Git.Revision, v.CI.Git.Branch, v.CI.Git.Remote
				}
			case "testRunStarted":
				var v struct{ Timestamp *messageTime }
				err = decode(b, &v)
				if err == nil && v.Timestamp != nil {
					out.Started, err = v.Timestamp.string()
				}
			case "testRunFinished":
				if runFinished {
					return MessageRun{}, ErrInvalid
				}
				runFinished = true
				var v struct {
					Exception *struct{ Type, Message, StackTrace string }
					Success   *bool
					Timestamp *messageTime
				}
				err = decode(b, &v)
				if err == nil && v.Timestamp != nil {
					out.Finished, err = v.Timestamp.string()
				}
				if v.Success != nil {
					runFailed = !*v.Success
				}
				if v.Exception != nil {
					err = ErrIncomplete
				}
			case "parseError":
				err = ErrInvalid
			case "testRunHookStarted":
				var v runHookStart
				err = decode(b, &v)
				if v.ID == "" || runHooks[v.ID].ID != "" {
					err = ErrInvalid
				}
				runHooks[v.ID] = v
				hookOrder = append(hookOrder, v.ID)
			case "testRunHookFinished":
				var v runHookFinish
				err = decode(b, &v)
				if v.TestRunHookStartedID == "" || runHookResults[v.TestRunHookStartedID].TestRunHookStartedID != "" {
					err = ErrInvalid
				}
				runHookResults[v.TestRunHookStartedID] = v
			case "externalAttachment":
				err = fmt.Errorf("%w: external evidence must be embedded for offline reporting", ErrInvalid)
			}
			if err != nil {
				return MessageRun{}, atPath(err, "message[%d]", line)
			}
		}
	}
	if reader.N == 0 {
		return MessageRun{}, atPath(fmt.Errorf("%w: input limit %d bytes", ErrTooLarge, lim.Input), "message[%d]", line)
	}
	if err := scanner.Err(); err != nil {
		return MessageRun{}, atPath(fmt.Errorf("%w: input limit %d bytes", ErrTooLarge, lim.Input), "message[%d]", line)
	}
	if !runFinished {
		for _, id := range order {
			if _, ok := finishes[id]; !ok {
				p := pickles[cases[starts[id].TestCaseID].PickleID]
				return MessageRun{}, &streamHint{fmt.Sprintf("Messages stream incomplete at %s: test case started but not finished; no testRunFinished", messageLocation(p, lines)), ErrIncomplete}
			}
		}
		return MessageRun{}, &streamHint{"Messages stream incomplete: no testRunFinished", ErrIncomplete}
	}
	pickleNames := map[string]map[string]int{}
	for _, p := range pickles {
		if pickleNames[p.URI] == nil {
			pickleNames[p.URI] = map[string]int{}
		}
		pickleNames[p.URI][p.Name]++
	}
	// Runner counters differ (Ruby is one-based and may report false retry flags).
	// The start-event order is authoritative within each test case.
	totals := map[string]int{}
	for _, start := range starts {
		totals[start.TestCaseID]++
	}
	ordinals := map[string]int{}
	for _, id := range order {
		start := starts[id]
		ordinal := ordinals[start.TestCaseID]
		ordinals[start.TestCaseID]++
		tc, ok := cases[start.TestCaseID]
		if !ok {
			return out, ErrIncomplete
		}
		p, ok := pickles[tc.PickleID]
		if !ok {
			return out, ErrIncomplete
		}
		finish, ok := finishes[id]
		if !ok {
			return out, &streamHint{fmt.Sprintf("Messages stream incomplete at %s: test case started but not finished", messageLocation(p, lines)), ErrIncomplete}
		}
		delete(finishes, id)
		begin, err := start.Timestamp.string()
		if err != nil {
			return out, err
		}
		end, err := finish.Timestamp.string()
		if err != nil {
			return out, err
		}
		if !messageIntervalValid(begin, end) {
			return out, ErrInvalid
		}
		sc := Scenario{ID: p.ID, Name: p.Name, Keyword: "Scenario", Type: "scenario"}
		modelmeta.ScenarioMetadata(&sc).IdentityNameCount = max(nameCatalog[p.URI][p.Name], pickleNames[p.URI][p.Name])
		if len(p.AstNodeIDs) > 1 {
			sc.Type = "scenario_outline"
			sc.Keyword = "Scenario Outline"
		}
		for _, aid := range p.AstNodeIDs {
			if lines[aid] > 0 {
				sc.Line = lines[aid]
			}
		}
		for _, tag := range p.Tags {
			sc.Tags = append(sc.Tags, Tag{Name: tag.Name})
		}
		ps := map[string]pickleStep{}
		for _, step := range p.Steps {
			ps[step.ID] = step
		}
		firstOwn, lastOwn := len(tc.TestSteps), -1
		for position, step := range tc.TestSteps {
			if step.PickleStepID != "" {
				firstOwn = min(firstOwn, position)
				lastOwn = position
			}
		}
		for position, ts := range tc.TestSteps {
			key := id + "\x00" + ts.ID
			res, exists := results[key]
			if !exists {
				if !runFailed {
					return out, &streamHint{fmt.Sprintf("Messages stream incomplete at %s:%d: finished case has no step result", p.URI, sc.Line), ErrIncomplete}
				}
				res.TestStepResult.Duration.Seconds = "0"
				res.TestStepResult.Status = "SKIPPED"
				if !modelmeta.ScenarioMetadata(&sc).MissingStepResults {
					sc.Before = append(sc.Before, Step{Name: "Runner reported no step result", Keyword: "Hook ", Result: Result{Status: StatusFailed, Error: "Runner reported no step result for this completed execution"}})
					modelmeta.ScenarioMetadata(&sc).MissingStepResults = true
				}
			}
			delete(results, key)
			status := messageStatus(res.TestStepResult.Status)
			if !status.Valid() {
				return out, ErrInvalid
			}
			d, err := res.TestStepResult.Duration.duration()
			if err != nil {
				return out, err
			}
			st := Step{Result: Result{Status: status, Error: messageError(res.TestStepResult), Duration: &d}}
			if ts.PickleStepID != "" {
				step, exists := ps[ts.PickleStepID]
				if !exists {
					return out, ErrIncomplete
				}
				if hasBackground[p.URI] && len(step.AstNodeIDs) == 0 {
					modelmeta.ScenarioMetadata(&sc).BackgroundUnresolved = true
				}
				modelmeta.StepMetadata(&st).Origin = "own"
				for _, aid := range step.AstNodeIDs {
					if backgroundSteps[aid] {
						modelmeta.StepMetadata(&st).Origin = "background"
					}
				}
				st.Name = step.Text
				st.Keyword = "Step "
				for _, aid := range step.AstNodeIDs {
					if keywords[aid] != "" {
						st.Keyword = keywords[aid]
					}
				}
				if doc := step.Argument.DocString; doc != nil {
					st.DocString = &DocString{Value: doc.Content, ContentType: doc.MediaType}
				}
				if table := step.Argument.DataTable; table != nil {
					for _, row := range table.Rows {
						r := Row{}
						for _, cell := range row.Cells {
							r.Cells = append(r.Cells, cell.Value)
						}
						st.Rows = append(st.Rows, r)
					}
				}
			} else if ts.HookID != "" {
				modelmeta.StepMetadata(&st).Origin = "hook"
				st.Keyword = "Hook "
				if kind := hookKinds[ts.HookID]; strings.Contains(kind, "after") {
					modelmeta.StepMetadata(&st).HookKind = "after"
				} else if strings.Contains(kind, "before") {
					modelmeta.StepMetadata(&st).HookKind = "before"
				}
				st.Name = hooks[ts.HookID]
				if st.Name == "" {
					st.Name = "Step hook"
					if position < firstOwn {
						st.Name = "before hook"
					} else if position > lastOwn {
						st.Name = "after hook"
					}
				}
				if modelmeta.StepMetadata(&st).HookKind == "" {
					if position < firstOwn {
						modelmeta.StepMetadata(&st).HookKind = "before"
					} else if position > lastOwn {
						modelmeta.StepMetadata(&st).HookKind = "after"
					}
				}
			} else {
				return out, ErrIncomplete
			}
			if len(ts.StepDefinitionIDs) > 0 {
				st.Match.Location = definitions[ts.StepDefinitionIDs[0]]
			}
			for _, a := range attachments[key] {
				data := a.Body
				if a.ContentEncoding == "IDENTITY" {
					data = base64.StdEncoding.EncodeToString([]byte(data))
				} else if a.ContentEncoding != "BASE64" {
					return out, ErrInvalid
				}
				if int64(len(data)) > lim.Embedding {
					return out, fmt.Errorf("%w: attachment limit %d base64 characters in execution %d", ErrTooLarge, lim.Embedding, len(out.Attempts)+1)
				}
				name := a.FileName
				if name == "" && a.MediaType == "text/plain" {
					name = "Runner output"
				}
				st.Embeddings = append(st.Embeddings, Embedding{Name: name, MimeType: a.MediaType, Data: data})
			}
			delete(attachments, key)
			sc.Steps = append(sc.Steps, st)
		}
		// Around hooks may attach before any testStepId exists. Ruby can also
		// reuse the preceding case's step ID; preserve these at case scope.
		var caseKeys []string
		for key := range attachments {
			if strings.HasPrefix(key, id+"\x00") {
				caseKeys = append(caseKeys, key)
			}
		}
		sort.Strings(caseKeys)
		if len(caseKeys) > 0 {
			st := Step{Name: "Around hook output", Keyword: "Hook ", Result: Result{Status: StatusPassed}}
			modelmeta.StepMetadata(&st).Origin = "hook"
			modelmeta.StepMetadata(&st).HookKind = "around"
			for _, key := range caseKeys {
				if key != id+"\x00" {
					modelmeta.ScenarioMetadata(&sc).CaseAttachments = true
				}
				for _, a := range attachments[key] {
					data := a.Body
					if a.ContentEncoding == "IDENTITY" {
						data = base64.StdEncoding.EncodeToString([]byte(data))
					} else if a.ContentEncoding != "BASE64" {
						return out, ErrInvalid
					}
					if int64(len(data)) > lim.Embedding {
						return out, ErrTooLarge
					}
					name := "Around hook output"
					if a.FileName != "" {
						name += " · " + a.FileName
					}
					st.Embeddings = append(st.Embeddings, Embedding{Name: name, MimeType: a.MediaType, Data: data})
				}
				delete(attachments, key)
			}
			sc.Before = append(sc.Before, st)
		}
		// A recorded retry proves that the previous execution did not pass,
		// even if an Around-after failure has no testStepFinished envelope.
		if ordinals[start.TestCaseID] < totals[start.TestCaseID] && modelmeta.ExecutionStatus(sc) == StatusPassed {
			sc.After = append(sc.After, Step{Name: "Runner retried an execution with no reported failed step", Keyword: "Hook ", Result: Result{Status: StatusFailed, Error: "Runner retried this execution; its failed hook or execution result was not emitted"}})
			modelmeta.ScenarioMetadata(&sc).InferredRetryFailure = true
		}
		if err := evidence.Check(caseFloors[id], modelmeta.Evidence([]Feature{{Elements: []Scenario{sc}}})); err != nil {
			return out, fmt.Errorf("%s:%d: %w", p.URI, sc.Line, err)
		}
		oracleExecution := evidence.Execution{}
		for _, part := range [][]Step{sc.Before, sc.Steps, sc.After} {
			for _, st := range part {
				oracleExecution.Results = append(oracleExecution.Results, string(st.Result.Status))
			}
		}
		oracleShown[id] = oracleExecution
		out.Attempts = append(out.Attempts, MessageAttempt{URI: p.URI, FeatureName: featureInfo[p.URI].Name, FeatureDescription: featureInfo[p.URI].Description, FeatureTags: featureInfo[p.URI].Tags, Scenario: sc, Number: ordinal, Started: begin, Finished: end, Worker: start.WorkerID})
		if finish.WillBeRetried && ordinals[start.TestCaseID] == totals[start.TestCaseID] {
			return out, &streamHint{fmt.Sprintf("Messages stream incomplete at %s:%d: promised retry has no following execution", p.URI, sc.Line), ErrIncomplete}
		}
	}
	for hookIndex, id := range hookOrder {
		start := runHooks[id]
		finish, ok := runHookResults[id]
		if !ok {
			return out, ErrIncomplete
		}
		begin, err := start.Timestamp.string()
		if err != nil {
			return out, err
		}
		end, err := finish.Timestamp.string()
		if err != nil {
			return out, err
		}
		delete(runHookResults, id)
		if !messageIntervalValid(begin, end) {
			return out, ErrInvalid
		}
		duration, err := finish.Result.Duration.duration()
		if err != nil {
			return out, err
		}
		status := messageStatus(finish.Result.Status)
		if !status.Valid() {
			return out, ErrInvalid
		}
		name := hooks[start.HookID]
		if name == "" {
			name = "Run setup / teardown"
		}
		st := Step{Keyword: "Hook ", Name: name, Result: Result{Status: status, Error: messageError(finish.Result), Duration: &duration}}
		modelmeta.StepMetadata(&st).Origin = "hook"
		for _, a := range attachments["run-hook\x00"+id] {
			data := a.Body
			if a.ContentEncoding == "IDENTITY" {
				data = base64.StdEncoding.EncodeToString([]byte(data))
			} else if a.ContentEncoding != "BASE64" {
				return out, ErrInvalid
			}
			if int64(len(data)) > lim.Embedding {
				return out, fmt.Errorf("%w: attachment limit %d base64 characters in execution %d", ErrTooLarge, lim.Embedding, len(out.Attempts)+1)
			}
			attachmentName := a.FileName
			if attachmentName == "" && a.MediaType == "text/plain" {
				attachmentName = "Runner output"
			}
			st.Embeddings = append(st.Embeddings, Embedding{Name: attachmentName, MimeType: a.MediaType, Data: data})
		}
		delete(attachments, "run-hook\x00"+id)
		sc := Scenario{Type: "run_hook", Line: hookIndex, ID: "run-hook-" + id, Name: name, Keyword: "Run hook", Steps: []Step{st}}
		out.Attempts = append(out.Attempts, MessageAttempt{URI: "__run_hooks__.feature", Scenario: sc, Number: 0, Started: begin, Finished: end, Worker: start.WorkerID})
	}
	var allEvidence []Feature
	for _, a := range out.Attempts {
		allEvidence = append(allEvidence, Feature{Elements: []Scenario{a.Scenario}})
	}
	if floorErr := evidence.Check(rawFloor, modelmeta.Evidence(allEvidence)); floorErr != nil {
		return out, floorErr
	}
	final := map[string]MessageAttempt{}
	for _, a := range out.Attempts {
		if err := Validate([]Feature{{URI: a.URI, Elements: []Scenario{a.Scenario}}}); err != nil {
			return out, err
		}
		key := a.URI + "\x00" + a.Scenario.ID
		if prev, ok := final[key]; !ok || a.Number > prev.Number {
			final[key] = a
		}
	}
	if len(results) > 0 || len(finishes) > 0 || len(attachments) > 0 || len(runHookResults) > 0 {
		return out, fmt.Errorf("cucumber: stream incomplete: execution events were not matched")
	}
	// The independent scanner is also authoritative for the runner failure signal.
	runFailed = runFailed || rawFloor.RunnerFailed
	if runFailed && len(out.Attempts) == 0 {
		return out, fmt.Errorf("cucumber: runner reported failure without execution results")
	}
	if len(oracleShown) == 0 && len(out.Attempts) > 0 {
		oracleShown["run-hooks"] = evidence.Execution{Results: []string{"failed"}}
	}
	if err := rawOracle.Check(oracleShown, runFailed); err != nil {
		return out, err
	}
	if len(out.Attempts) > 0 {
		modelmeta.ScenarioMetadata(&out.Attempts[0].Scenario).RawRunnerAudit = rawOracle.Audit().Encode()
	}
	rawCases, _ := evidence.CaseAudits(rawOracle.Audit().Encode())
	for i := range out.Attempts {
		a := &out.Attempts[i]
		modelmeta.ScenarioMetadata(&a.Scenario).RawCaseAudit = rawCases[evidence.LocationKey(a.URI, a.Scenario.Line, a.Scenario.Name)]
		out.Attempts[i].RunnerFailed = runFailed
		modelmeta.ScenarioMetadata(&out.Attempts[i].Scenario).RunnerFailed = runFailed
	}
	sort.SliceStable(out.Attempts, func(i, j int) bool { return out.Attempts[i].Number < out.Attempts[j].Number })
	for i := range out.Attempts {
		a := &out.Attempts[i]
		catalog := nameCatalog[a.URI]
		if len(catalog) == 0 {
			catalog = pickleNames[a.URI]
		}
		modelmeta.SetAttemptCatalog(a, catalog)
	}

	return out, nil
}

func collectMessageLines(v any, lines map[string]int) {
	switch x := v.(type) {
	case map[string]any:
		if id, ok := x["id"].(string); ok {
			if loc, ok := x["location"].(map[string]any); ok {
				if n, ok := loc["line"].(json.Number); ok {
					line, _ := strconv.Atoi(string(n))
					lines[id] = line
				}
			}
		}
		for _, v := range x {
			collectMessageLines(v, lines)
		}
	case []any:
		for _, v := range x {
			collectMessageLines(v, lines)
		}
	}
}

func messageIntervalValid(start, end string) bool {
	a, e1 := time.Parse(time.RFC3339Nano, start)
	b, e2 := time.Parse(time.RFC3339Nano, end)
	return e1 == nil && e2 == nil && !b.Before(a)
}

// UNKNOWN is a valid Messages value with no established outcome. Pending is the
// conservative legacy display category; the original status remains explicit in evidence.
func messageStatus(raw string) Status {
	if raw == "UNKNOWN" {
		return StatusPending
	}
	return Status(strings.ToLower(raw))
}

func messageError(r messageStepResult) string {
	if r.Status == "UNKNOWN" {
		return "Cucumber Messages status: UNKNOWN (outcome not established).\n" + r.errorText()
	}
	return r.errorText()
}

// Background AST step IDs distinguish inherited pickle steps from scenario-owned steps.
func collectBackgroundSteps(v any, ids map[string]bool) {
	switch x := v.(type) {
	case map[string]any:
		if bg, ok := x["background"].(map[string]any); ok {
			if steps, ok := bg["steps"].([]any); ok {
				for _, s := range steps {
					if st, ok := s.(map[string]any); ok {
						if id, ok := st["id"].(string); ok {
							ids[id] = true
						}
					}
				}
			}
		}
		for _, child := range x {
			collectBackgroundSteps(child, ids)
		}
	case []any:
		for _, child := range x {
			collectBackgroundSteps(child, ids)
		}
	}
}

func collectStepKeywords(v any, keywords map[string]string) {
	switch x := v.(type) {
	case []any:
		for _, c := range x {
			collectStepKeywords(c, keywords)
		}
	case map[string]any:
		if id, ok := x["id"].(string); ok {
			if keyword, ok := x["keyword"].(string); ok {
				keywords[id] = keyword
			}
		}
		for _, c := range x {
			collectStepKeywords(c, keywords)
		}
	}
}

func messageLocation(p messagePickle, lines map[string]int) string {
	line := 0
	for _, id := range p.AstNodeIDs {
		if lines[id] > 0 {
			line = lines[id]
		}
	}
	return fmt.Sprintf("%s:%d", p.URI, line)
}
