package evidence

// Stream is an immutable observation of an input before merging, independent of
// report identities, retry folding and outcome classification.
type Stream struct {
	Name        string
	Failed      bool
	Unexplained []string
	Cases       map[string][]Execution
	Locations   []string
}

// Execution records raw results in source order. Only an entirely passed final
// execution can rescue an earlier failure; skipped and pending are not passes.
type Execution struct {
	Results []string
	// NoOwnSteps distinguishes an empty scenario with passing hooks from a pass.
	NoOwnSteps     bool
	MissingResults bool
}

// Final is the report's claimed outcome and the attempt evidence it displays.
type Final struct {
	Status   string
	Attempts []Execution
}

func strictlyPassed(e Execution) bool {
	if e.NoOwnSteps || len(e.Results) == 0 {
		return false
	}
	for _, s := range e.Results {
		if s != "passed" {
			return false
		}
	}
	return true
}

func hard(e Execution) bool {
	for _, s := range e.Results {
		if s == "failed" || s == "ambiguous" {
			return true
		}
	}
	return false
}

// CheckInputs independently enforces invariant c before transformations can erase
// ownership. Locations are raw observations, not computed report identities.
func CheckInputs(streams []Stream) error {
	seen := map[string]int{}
	for i, stream := range streams {
		for _, location := range stream.Locations {
			if owner, ok := seen[location]; ok && owner != i {
				return Floorf("evidence floor: cross-input repeat in inputs %q and %q; this is a reporter bug", streams[owner].Name, stream.Name)
			}
			seen[location] = i
		}
	}
	return nil
}

// CheckOutcomes is a separate runtime oracle. It only consumes raw observations
// and claimed values; it imports no parser, renderer, or decision implementation.
func CheckOutcomes(streams []Stream, final map[string]Final, _ bool) error {
	if err := CheckInputs(streams); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, stream := range streams {
		for key, raw := range stream.Cases {
			if seen[key] {
				return Floorf("evidence floor: cross-input repeat; this is a reporter bug")
			}
			seen[key] = true
			shown, ok := final[key]
			if !ok {
				return Floorf("evidence floor: input execution disappeared; this is a reporter bug")
			}
			if len(shown.Attempts) < len(raw) {
				return Floorf("evidence floor: retry evidence disappeared; this is a reporter bug")
			}
			if len(raw) > 1 {
				nonpassed := false
				for _, a := range shown.Attempts[:len(shown.Attempts)-1] {
					nonpassed = nonpassed || !strictlyPassed(a)
				}
				if !nonpassed {
					return Floorf("evidence floor: repeated execution has no non-passed attempt; this is a reporter bug")
				}
			}
			// In-input retries may rescue a prior failure only if the RAW final
			// execution passed. A rewritten displayed attempt cannot grant rescue.
			rawFailure := false
			for _, attempt := range raw {
				rawFailure = rawFailure || hard(attempt)
			}
			if rawFailure && !strictlyPassed(raw[len(raw)-1]) && shown.Status != "failed" && shown.Status != "ambiguous" && shown.Status != "known" {
				return Floorf("evidence floor: hard result became green without a passed final execution; this is a reporter bug")
			}
		}
	}
	return nil
}
