package cucumbermodel

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// FormatDuration formats d for people, following multiple-cucumber-html-reporter's human-readable
// format: "0s" for zero, whole milliseconds below one second ("40ms", "<1ms"), and otherwise days,
// hours, minutes and whole seconds with zero parts left out ("1h 2m 3s").
func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	if d < time.Second {
		ms := math.Round(float64(d) / float64(time.Millisecond))
		if ms == 0 {
			return "<1ms"
		}
		return strconv.FormatFloat(ms, 'f', 0, 64) + "ms"
	}
	total := int64(d / time.Second)
	parts := make([]string, 0, 4)
	for _, u := range []struct {
		size   int64
		suffix string
	}{{86400, "d"}, {3600, "h"}, {60, "m"}} {
		if n := total / u.size; n > 0 {
			parts = append(parts, strconv.FormatInt(n, 10)+u.suffix)
		}
		total %= u.size
	}
	if total > 0 || len(parts) == 0 {
		parts = append(parts, strconv.FormatInt(total, 10)+"s")
	}
	return strings.Join(parts, " ")
}

// Duration is the sum of the scenario's step durations. Steps without a duration count as zero.
func (s Scenario) Duration() time.Duration {
	var total time.Duration
	for _, st := range s.Steps {
		if st.Result.Duration != nil {
			total = addDuration(total, *st.Result.Duration)
		}
	}
	return total
}

// Duration is the sum of the feature's scenario durations.
func (f Feature) Duration() time.Duration {
	var total time.Duration
	for _, sc := range f.Elements {
		total = addDuration(total, sc.Duration())
	}
	return total
}

// addDuration adds non-negative durations, saturating instead of overflowing.
func addDuration(a, b time.Duration) time.Duration {
	if b > math.MaxInt64-a {
		return math.MaxInt64
	}
	return a + b
}
