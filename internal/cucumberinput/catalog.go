package cucumber

import "strings"

// collectScenarioNames expands every AST row, including rows outside the selected
// pickle set and scenarios nested in Rules. Display keywords do not participate.
func collectScenarioNames(v any, counts map[string]int) {
	switch x := v.(type) {
	case map[string]any:
		if sc, ok := x["scenario"].(map[string]any); ok {
			name, _ := sc["name"].(string)
			examples, _ := sc["examples"].([]any)
			if len(examples) == 0 {
				counts[name]++
			} else {
				for _, value := range examples {
					ex, ok := value.(map[string]any)
					if !ok {
						continue
					}
					header, _ := ex["tableHeader"].(map[string]any)
					keys := cellValues(header)
					rows, _ := ex["tableBody"].([]any)
					for _, value := range rows {
						row, _ := value.(map[string]any)
						values := cellValues(row)
						rowName := name
						for i, key := range keys {
							if i < len(values) {
								rowName = strings.ReplaceAll(rowName, "<"+key+">", values[i])
							}
						}

						counts[rowName]++
					}
				}
			}
		}
		for _, child := range x {
			collectScenarioNames(child, counts)
		}
	case []any:
		for _, child := range x {
			collectScenarioNames(child, counts)
		}
	}
}

func cellValues(row map[string]any) []string {
	cells, _ := row["cells"].([]any)
	out := make([]string, 0, len(cells))
	for _, value := range cells {
		cell, _ := value.(map[string]any)
		s, _ := cell["value"].(string)
		out = append(out, s)
	}
	return out
}

func documentHasBackground(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		if _, ok := x["background"].(map[string]any); ok {
			return true
		}
		for _, child := range x {
			if documentHasBackground(child) {
				return true
			}
		}
	case []any:
		for _, child := range x {
			if documentHasBackground(child) {
				return true
			}
		}
	}
	return false
}
