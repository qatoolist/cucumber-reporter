package report

// sectionView is a caller section ready for the template. Tone is always one of the four tones.
type sectionView struct {
	ID, Title, Tone string
	Hidden          bool
	Summary, Rows   []Fact
	Note            string
}

func toneOrDefault(t string) string {
	if t == "" {
		return ToneInfo
	}
	return t
}

func cleanFacts(facts []Fact) []Fact {
	out := make([]Fact, len(facts))
	for i, f := range facts {
		out[i] = Fact{Label: clean(f.Label), Value: clean(f.Value)}
	}
	return out
}

func buildSection(s Section, hidden bool) sectionView {
	return sectionView{
		ID: s.ID, Title: clean(s.Title), Tone: toneOrDefault(s.Tone), Hidden: hidden,
		Summary: cleanFacts(s.Summary), Rows: cleanFacts(s.Rows), Note: clean(s.Note),
	}
}
