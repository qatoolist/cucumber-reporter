package report

import (
	"regexp"
	"strconv"
	"strings"

	modelmeta "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"

	cucumber "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
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
func unique(base string, seen map[string]bool, next map[string]int) string {
	id := base
	n := max(2, next[base])
	for seen[id] {
		id = base + "-" + strconv.Itoa(n)
		n++
	}
	next[base] = n
	seen[id] = true
	return id
}

// featureIDs returns the stable identifier of each feature, as used in the page's links
// (#/features/<id>): the slug of the URI's base name, with -2, -3 … appended on collisions. The same
// run always yields the same identifiers. Synthetic run-hook features use "run-hooks",
// after ordinary feature slugs have been reserved.
func featureIDs(features []cucumber.Feature) []string {
	seen := map[string]bool{}
	next := map[string]int{}
	out := make([]string, len(features))
	// Reserve ordinary feature slugs first so synthetic hook records cannot rename
	// a real feature. Hook names must not depend on shard directory names.
	isHook := func(f cucumber.Feature) bool {
		if len(f.Elements) == 0 {
			return false
		}
		for _, sc := range f.Elements {
			if sc.Type != "run_hook" {
				return false
			}
		}
		return true
	}
	for i, f := range features {
		if modelmeta.FeatureMetadata(&f).ReportID != "" {
			out[i] = unique(modelmeta.FeatureMetadata(&f).ReportID, seen, next)
		} else if !isHook(f) {
			out[i] = unique(featureSlug(f.URI), seen, next)
		}
	}
	for i, f := range features {
		if modelmeta.FeatureMetadata(&f).ReportID == "" && isHook(f) {
			out[i] = unique("run-hooks", seen, next)
		}
	}
	return out
}

// scenarioIDs returns the stable identifier of each scenario, indexed like features[i].Elements[j]:
// "<feature id>-L<line>", with -2, -3 … appended when a feature has several scenarios on one line
// (outline rows). Annotation.Scenario takes these values; the page links to #/scenarios/<id>.
func scenarioIDs(features []cucumber.Feature) [][]string {
	fids := featureIDs(features)
	out := make([][]string, len(features))
	for i, f := range features {
		seen := map[string]bool{}
		next := map[string]int{}
		out[i] = make([]string, len(f.Elements))
		for j, sc := range f.Elements {
			base := fids[i] + "-L" + strconv.Itoa(sc.Line)
			if suffix := modelmeta.ScenarioMetadata(&sc).NavigationSuffix; suffix != "" && modelmeta.FeatureMetadata(&f).ReportID == "" {
				out[i][j] = fids[i] + suffix
				continue
			}
			if modelmeta.FeatureMetadata(&f).ReportID != "" && sc.Line == 0 {
				base += "-" + strings.TrimPrefix(Identity(f.URI, sc.Name), "test-")[:12]
			}
			out[i][j] = unique(base, seen, next)
		}
	}
	return out
}
