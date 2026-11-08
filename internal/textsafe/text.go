// Package textsafe neutralizes terminal controls in human-readable report text.
package textsafe

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// Hidden lists the byte spans of s that render as nothing: ANSI SGR coloring,
// which Display deletes, and zero-width or soft-hyphen characters, which a
// browser does not draw. Text either side of them reads as adjacent.
func Hidden(s []byte) [][2]int {
	var out [][2]int
	if bytes.Contains(s, []byte("\x1b[")) {
		for _, m := range sgr.FindAllIndex(s, -1) {
			out = append(out, [2]int{m[0], m[1]})
		}
	}
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRune(s[i:])
		if r == 0x00ad || r == 0x200b || r == 0x200c || r == 0x200d || r == 0x2060 || r == 0xfeff {
			out = append(out, [2]int{i, i + n})
		}
		i += n
	}
	slices.SortFunc(out, func(a, b [2]int) int { return a[0] - b[0] })
	return out
}

// Display strips ANSI SGR coloring and writes other C0/bidi formatting controls
// visibly. Newlines and tabs remain available for code blocks and CSV fields.
func Display(s string) string {
	if strings.Contains(s, "\x1b[") {
		s = sgr.ReplaceAllString(s, "")
	}
	dirty := !utf8.ValidString(s)
	for _, r := range s {
		if (r < 32 && r != '\n' && r != '\t' && r != '\r') || r == 127 || r == 0x200e || r == 0x200f || (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069) {
			dirty = true
			break
		}
	}
	if !dirty {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if (r < 32 && r != '\n' && r != '\t' && r != '\r') || r == 127 || r == 0x200e || r == 0x200f || (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069) {
			fmt.Fprintf(&b, "\\u%04x", r)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
