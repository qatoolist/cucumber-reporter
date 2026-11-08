package cucumber

import (
	modelmeta "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"
)

func fromInternal(features []modelmeta.Feature) []Feature {
	if features == nil {
		return nil
	}
	out := make([]Feature, len(features))
	for i, f := range features {
		out[i] = Feature{URI: f.URI, ID: f.ID, Keyword: f.Keyword, Name: f.Name, Description: f.Description, Line: f.Line, Tags: publicTags(f.Tags)}
		out[i].Provenance = publicFeatureProvenance(modelmeta.FeatureMetadata(&f))
		if f.Elements != nil {
			out[i].Elements = make([]Scenario, len(f.Elements))
		}
		for j, s := range f.Elements {
			out[i].Elements[j] = publicScenario(s)
		}
	}
	return out
}

func publicTags(tags []modelmeta.Tag) []Tag {
	if tags == nil {
		return nil
	}
	out := make([]Tag, len(tags))
	for i, t := range tags {
		out[i] = Tag(t)
	}
	return out
}

func publicScenario(s modelmeta.Scenario) Scenario {
	out := Scenario{ID: s.ID, Keyword: s.Keyword, Name: s.Name, Description: s.Description, Type: s.Type, Line: s.Line, Tags: publicTags(s.Tags), Steps: publicSteps(s.Steps, 0), Before: publicSteps(s.Before, 0), After: publicSteps(s.After, 0)}
	out.Provenance = publicScenarioProvenance(modelmeta.ScenarioMetadata(&s))
	return out
}

func publicSteps(steps []modelmeta.Step, depth int) []Step {
	if steps == nil {
		return nil
	}
	if depth > 64 {
		return []Step{{Result: Result{Status: StatusFailed, Error: "Hook nesting exceeds limit"}}}
	}
	out := make([]Step, len(steps))
	for i, s := range steps {
		t := Step{Keyword: s.Keyword, Name: s.Name, Line: s.Line, Match: Match(s.Match), Result: Result{Status: Status(s.Result.Status), Error: s.Result.Error, Duration: s.Result.Duration}, Before: publicSteps(s.Before, depth+1), After: publicSteps(s.After, depth+1)}
		t.Hidden = modelmeta.StepMetadata(&s).Origin == "hook"
		if s.DocString != nil {
			d := DocString(*s.DocString)
			t.DocString = &d
		}
		if s.Rows != nil {
			t.Rows = make([]Row, len(s.Rows))
			for j, r := range s.Rows {
				t.Rows[j] = Row(r)
			}
		}
		if s.Embeddings != nil {
			t.Embeddings = make([]Embedding, len(s.Embeddings))
			for j, e := range s.Embeddings {
				t.Embeddings[j] = Embedding(e)
			}
		}
		out[i] = t
		out[i].Provenance = publicStepProvenance(modelmeta.StepMetadata(&s))
	}
	return out
}
