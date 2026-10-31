package report

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/qatoolist/cucumber-reporter/cucumber"
)

var (
	lineSuffix = regexp.MustCompile(`:\d+$`)
	nonSlug    = regexp.MustCompile(`[^a-z0-9]+`)
)

// featureSlug is the lower-case base name of a feature URI without a ":line" suffix or extension, with
// every run of other characters replaced by "-"; "feature" when nothing is left.
func featureSlug(uri string) string {
	uri = lineSuffix.ReplaceAllString(uri, "")
	if i := strings.LastIndexAny(uri, `/\`); i >= 0 {
		uri = uri[i+1:]
	}
	if i := strings.LastIndexByte(uri, '.'); i > 0 {
		uri = uri[:i]
	}
	s := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(uri), "-"), "-")
	if s == "" {
		return "feature"
	}
	return s
}

// unique returns base, or base-2, base-3 … , the first that is not in seen, and records it.
func unique(base string, seen map[string]bool) string {
	id := base
	for n := 2; seen[id]; n++ {
		id = base + "-" + strconv.Itoa(n)
	}
	seen[id] = true
	return id
}

// FeatureIDs returns the stable identifier of each feature, as used in the page's links
// (#/features/<id>): the slug of the URI's base name, with -2, -3 … appended on collisions. The same
// run always yields the same identifiers.
func FeatureIDs(features []cucumber.Feature) []string {
	seen := map[string]bool{}
	out := make([]string, len(features))
	for i, f := range features {
		out[i] = unique(featureSlug(f.URI), seen)
	}
	return out
}

// ScenarioIDs returns the stable identifier of each scenario, indexed like features[i].Elements[j]:
// "<feature id>-L<line>", with -2, -3 … appended when a feature has several scenarios on one line
// (outline rows). Annotation.Scenario takes these values; the page links to #/scenarios/<id>.
func ScenarioIDs(features []cucumber.Feature) [][]string {
	fids := FeatureIDs(features)
	out := make([][]string, len(features))
	for i, f := range features {
		seen := map[string]bool{}
		out[i] = make([]string, len(f.Elements))
		for j, sc := range f.Elements {
			out[i][j] = unique(fids[i]+"-L"+strconv.Itoa(sc.Line), seen)
		}
	}
	return out
}
