package cucumbermodel

import (
	"maps"
	"slices"
)

// CloneFeatures copies execution evidence and private metadata without exposing
// bookkeeping through public JSON marshaling methods.
func CloneFeatures(features []Feature) ([]Feature, error) {
	out := slices.Clone(features)
	for i, f := range out {
		out[i].featureState.IdentityCatalog = maps.Clone(f.featureState.IdentityCatalog)
		out[i].Tags = slices.Clone(f.Tags)
		out[i].Elements = slices.Clone(f.Elements)
		for j, sc := range f.Elements {
			var err error
			sc.Tags = slices.Clone(sc.Tags)
			sc.Before, err = CloneSteps(sc.Before)
			if err != nil {
				return nil, err
			}
			sc.After, err = CloneSteps(sc.After)
			if err != nil {
				return nil, err
			}
			sc.Steps, err = CloneSteps(sc.Steps)
			if err != nil {
				return nil, err
			}
			out[i].Elements[j] = sc
		}
	}
	return out, nil
}

// CloneSteps deep-copies evidence, including hook trees, with a bounded depth.
func CloneSteps(steps []Step) ([]Step, error) { return cloneSteps(steps, 0) }

func cloneSteps(steps []Step, depth int) ([]Step, error) {
	if depth > 64 {
		return nil, ErrInvalid
	}
	out := slices.Clone(steps)
	for i, st := range out {
		var err error
		st.Before, err = cloneSteps(st.Before, depth+1)
		if err != nil {
			return nil, err
		}
		st.After, err = cloneSteps(st.After, depth+1)
		if err != nil {
			return nil, err
		}
		if st.DocString != nil {
			doc := *st.DocString
			st.DocString = &doc
		}
		if st.Result.Duration != nil {
			duration := *st.Result.Duration
			st.Result.Duration = &duration
		}
		st.Embeddings = slices.Clone(st.Embeddings)
		st.Rows = slices.Clone(st.Rows)
		for j := range st.Rows {
			st.Rows[j].Cells = slices.Clone(st.Rows[j].Cells)
		}
		out[i] = st
	}
	return out, nil
}
