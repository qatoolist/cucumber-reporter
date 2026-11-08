package report

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	input "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

// rejectRepeatedInputs inventories raw locations before any normalization. In-file
// retry duplicates belong to one input and remain supported.
func rejectRepeatedInputs(inputs []runInput) error {
	type location struct {
		uri  string
		line int
		name string
	}
	locations := map[location]map[int]bool{}
	for i, in := range inputs {
		add := func(uri string, s input.Scenario) {
			if strings.EqualFold(s.Type, "background") || strings.EqualFold(s.Type, "run_hook") {
				return
			}
			key := location{uri: normalizedURI(uri), line: s.Line}
			if s.Line <= 0 {
				key.name = s.Name
			}
			if locations[key] == nil {
				locations[key] = map[int]bool{}
			}
			locations[key][i] = true
		}
		for _, f := range in.Features {
			for _, s := range f.Elements {
				add(f.URI, s)
			}
		}
		for _, a := range in.Attempts {
			add(a.URI, a.Scenario)
		}
	}
	var keys []location
	for loc, owners := range locations {
		if len(owners) > 1 {
			keys = append(keys, loc)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	sort.Slice(keys, func(a, b int) bool {
		if keys[a].uri != keys[b].uri {
			return keys[a].uri < keys[b].uri
		}
		if keys[a].line != keys[b].line {
			return keys[a].line < keys[b].line
		}
		return keys[a].name < keys[b].name
	})
	total := len(keys)
	if len(keys) > 20 {
		keys = keys[:20]
	}
	collisions := make([]string, 0, len(keys)+1)
	for _, loc := range keys {
		var owners []int
		for i := range locations[loc] {
			owners = append(owners, i)
		}
		sort.Ints(owners)
		names := make([]string, len(owners))
		for n, i := range owners {
			names[n] = fmt.Sprintf("input[%d] %q", i, boundedText(inputs[i].Name, 80))
		}
		entry := fmt.Sprintf("%q:%d", boundedText(loc.uri, 120), loc.line)
		if loc.line <= 0 {
			entry += fmt.Sprintf(" (%q)", boundedText(loc.name, 60))
		}
		collisions = append(collisions, entry+" in "+strings.Join(names, ", "))
	}
	if total > len(collisions) {
		collisions = append(collisions, fmt.Sprintf("and %d more", total-len(collisions)))
	}
	prefix := ""
	if same := identicalInputs(inputs); same != "" {
		prefix = same + "; "
	}
	return invalidInput("%srepeated scenarios across inputs: %s; merging reruns or matrix cells across inputs is not supported in v0.3.0; create one report per run or cell, or use --retry within a single run", prefix, strings.Join(collisions, "; "))
}

// identicalInputs names the first input whose parsed content equals an earlier
// input's, such as the same results file copied to two paths. It runs only after
// a collision is found, so disjoint shards pay no hashing cost.
func identicalInputs(inputs []runInput) string {
	seen := map[[sha256.Size]byte]int{}
	for i, in := range inputs {
		if len(in.Features) == 0 && len(in.Attempts) == 0 {
			continue
		}
		h := sha256.New()
		if err := json.NewEncoder(h).Encode(struct {
			Features []input.Feature
			Attempts []input.MessageAttempt
		}{in.Features, in.Attempts}); err != nil {
			continue
		}
		var sum [sha256.Size]byte
		copy(sum[:], h.Sum(nil))
		if first, ok := seen[sum]; ok {
			return fmt.Sprintf("input[%d] %q has the same content as input[%d] %q", i, boundedText(in.Name, 80), first, boundedText(inputs[first].Name, 80))
		}
		seen[sum] = i
	}
	return ""
}

// boundedText shortens one value of a refusal at a UTF-8 boundary, so the
// surrounding line numbers and input names stay visible.
func boundedText(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	end := limit - 1
	for end > 0 && !utf8.ValidString(s[:end]) {
		end--
	}
	return s[:end] + "…"
}
