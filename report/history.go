package report

import (
	"container/heap"
	"encoding/json"
	"fmt"
	"sort"
)

// HistoryByteLimit is shared with CLI readers so every persisted history can be re-read.
const HistoryByteLimit = 64 << 20

func scopeKey(s RunSummary) string {
	b, _ := json.Marshal([]string{s.Environment, s.Branch})
	return string(b)
}

func runKey(s RunSummary) string {
	b, _ := json.Marshal([]string{s.Environment, s.Branch, s.ID})
	return string(b)
}

func archiveSummary(s RunSummary) RunSummary {
	out := RunSummary{GeneratedID: s.GeneratedID, Version: s.Version, ID: s.ID, Environment: s.Environment, Branch: s.Branch, RecordedAt: s.RecordedAt, Counts: s.Counts, IssueRecords: s.IssueRecords}
	out.Scenarios = make([]ScenarioSummary, len(s.Scenarios))
	for i, x := range s.Scenarios {
		out.Scenarios[i] = ScenarioSummary{GeneratedIdentity: x.GeneratedIdentity, Identity: x.Identity, Status: x.Status, Duration: x.Duration}
	}
	return out
}

type (
	historyArchive struct {
		entries     []RunSummary
		analysis    []RunSummary
		diagnostics []Diagnostic
		omitted     bool
	}
	scopeBudget struct {
		key     string
		bytes   int
		indices []int
		next    int
	}
	budgetHeap []*scopeBudget
)

func (h budgetHeap) Len() int { return len(h) }
func (h budgetHeap) Less(i, j int) bool {
	if h[i].bytes == h[j].bytes {
		return h[i].key < h[j].key
	}
	return h[i].bytes > h[j].bytes
}
func (h budgetHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }

// Push implements heap.Interface.
func (h *budgetHeap) Push(x any) { *h = append(*h, x.(*scopeBudget)) }

// Pop implements heap.Interface.
func (h *budgetHeap) Pop() any { old := *h; x := old[len(old)-1]; *h = old[:len(old)-1]; return x }

// Retain per scope for analysis, then budget only the published archive. Prune
// older observations first, then other scopes; preserve the current newest run
// whenever it fits. Otherwise omit the optional archive.
func compactHistory(history []RunSummary, retain, budget int, current ...string) (historyArchive, error) {
	result := historyArchive{}
	counts := map[string]int{}
	keep := make([]bool, len(history))
	pruned := map[string]int{}
	for i := len(history) - 1; i >= 0; i-- {
		key := scopeKey(history[i])
		if counts[key] < retain {
			keep[i] = true
			counts[key]++
		} else {
			pruned[key]++
		}
	}
	sizes := []int{}
	scopes := map[string]*scopeBudget{}
	total := 3
	for i, s := range history {
		if !keep[i] {
			continue
		}
		if err := validateSummary(s); err != nil {
			return result, err
		}
		a := archiveSummary(s)
		b, err := json.Marshal(a)
		if err != nil {
			return result, err
		}
		key := scopeKey(s)
		scope := scopes[key]
		if scope == nil {
			scope = &scopeBudget{key: key}
			scopes[key] = scope
		}
		scope.indices = append(scope.indices, len(result.entries))
		scope.bytes += len(b) + 1
		result.entries = append(result.entries, a)
		sizes = append(sizes, len(b)+1)
		total += len(b) + 1
	}
	result.analysis = append([]RunSummary(nil), result.entries...)
	currentScope := ""
	if len(current) > 0 {
		currentScope = current[0]
	} else if len(history) > 0 {
		currentScope = scopeKey(history[len(history)-1])
	}
	q := budgetHeap{}
	for _, s := range scopes {
		if s.key != currentScope && len(s.indices) > 1 {
			q = append(q, s)
		}
	}
	heap.Init(&q)
	for total > budget && q.Len() > 0 {
		scope := heap.Pop(&q).(*scopeBudget)
		index := scope.indices[scope.next]
		scope.next++
		total -= sizes[index]
		scope.bytes -= sizes[index]
		sizes[index] = 0
		pruned[scope.key]++
		if len(scope.indices)-scope.next > 1 {
			heap.Push(&q, scope)
		}
	}
	// Preserve the current scope when a different scope's newest snapshot alone
	// is too large. This loss is explicit, rather than disabling every archive.
	var other []*scopeBudget
	for key, scope := range scopes {
		if key != currentScope {
			other = append(other, scope)
		}
	}
	sort.Slice(other, func(i, j int) bool {
		if other[i].bytes == other[j].bytes {
			return other[i].key < other[j].key
		}
		return other[i].bytes > other[j].bytes
	})
	for _, scope := range other {
		if total <= budget {
			break
		}
		for _, index := range scope.indices {
			total -= sizes[index]
			sizes[index] = 0
		}
		result.diagnostics = append(result.diagnostics, Diagnostic{Path: "history", Message: "Dropped archived scope " + boundedScope(scope.key) + " to meet the reader byte budget"})
	}
	// Only after other scopes have been exhausted may the current scope lose older snapshots.
	if scope := scopes[currentScope]; scope != nil {
		for total > budget && scope.next < len(scope.indices)-1 {
			index := scope.indices[scope.next]
			scope.next++
			total -= sizes[index]
			sizes[index] = 0
			pruned[scope.key]++
		}
	}
	if total > budget {
		result.omitted = true
		result.entries = nil
		result.diagnostics = append(result.diagnostics, Diagnostic{Path: "history", Message: "history.json omitted: the remaining newest snapshot exceeds the reader byte budget; summary.json and the report are retained"})
		return result, nil
	}
	keys := make([]string, 0, len(pruned))
	for key := range pruned {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		result.diagnostics = append(result.diagnostics, Diagnostic{Path: "history", Message: fmt.Sprintf("Pruned %d older snapshots for environment/branch %s to retention and reader byte budget", pruned[key], boundedScope(key))})
	}
	kept := result.entries[:0]
	for i, s := range result.entries {
		if sizes[i] > 0 {
			kept = append(kept, s)
		}
	}
	result.entries = kept
	return result, nil
}

func boundedScope(s string) string {
	r := []rune(s)
	if len(r) > 120 {
		return string(r[:120]) + "…"
	}
	return s
}
