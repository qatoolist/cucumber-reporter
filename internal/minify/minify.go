// Package minify is a dependency-free, conservative JavaScript and CSS minifier
// used to build the embedded report assets.
//
// JavaScript is processed by a single-pass tokenizer that understands strings,
// template literals (with nested ${...}), regular expression literals (decided
// by the previous significant token) and comments. Comments are removed, a
// whitespace run containing a line break becomes one "\n" (so automatic
// semicolon insertion is unchanged), and any other run becomes one space only
// where two word characters, "+ +" or "- -" would otherwise join. Identifiers
// are never renamed.
//
// CSS comments are removed, whitespace collapses to one space outside strings,
// and spaces are dropped around { } ; , > and after ":" (never before it, so
// descendant ":hover" selectors keep their meaning). Operators inside calc()
// are untouched, so their spaces survive.
//
// Output is deterministic and every successful result ends with exactly one
// newline. Errors never include any of the input text.
package minify

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// regexAfterWords are the keywords after which a "/" starts a regular
// expression rather than a division.
func regexAfterWord(w string) bool {
	switch w {
	case "return", "typeof", "instanceof", "in", "of", "new", "delete",
		"void", "throw", "case", "do", "else", "yield", "await":
		return true
	}
	return false
}

// isWord reports whether r is a JavaScript word character for joining
// purposes: ASCII letters, digits, "_", "$", "\" and U+0080 to U+FFFF.
func isWord(r rune) bool {
	if isJSSpace(r) {
		return false
	}
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	case r == '_', r == '$', r == '\\':
		return true
	}
	return r >= 0x80 && r <= 0xffff
}

func isJSSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\r', '\n', '\f', '\v', '\u00a0', '\uFEFF', '\u2028', '\u2029':
		return true
	}
	return unicode.Is(unicode.Zs, r)
}

// isSpace matches the characters that trim and the regexp \s class treat as
// whitespace: Unicode spaces plus the C0 separators U+001C to U+001F.
func isSpace(r rune) bool {
	return unicode.IsSpace(r) || (r >= 0x1c && r <= 0x1f)
}

func decode(src string) ([]rune, error) {
	if !utf8.ValidString(src) {
		return nil, fmt.Errorf("minify: input is not valid UTF-8")
	}
	return []rune(src), nil
}

func finish(s string) string {
	return strings.TrimFunc(s, isSpace) + "\n"
}

// JS minifies JavaScript source. It returns an error for an unterminated
// string, template literal, regular expression or comment, or for input that
// is not valid UTF-8.
func JS(src string) (string, error) {
	s, err := decode(src)
	if err != nil {
		return "", err
	}
	n := len(s)
	var out strings.Builder
	var prev rune   // last rune written to out
	var last string // last significant token, for regex detection
	pending := rune(0)
	var braces []bool // true marks a template expression brace
	var controlParens []bool
	afterControl := false
	afterBlock := false
	lastRegex := false
	var blocks []bool
	classPending, classStatement := false, false

	emit := func(tok []rune) {
		first := tok[0]
		if pending != 0 && out.Len() > 0 {
			switch {
			case pending == '\n':
				out.WriteByte('\n')
			case (isWord(prev) && isWord(first)) || ((prev == '+' || prev == '-') && first == prev),
				// A division followed by "*" or "/" would otherwise fuse into a
				// comment opener. The reference minifier misses this.
				prev == '/' && (first == '/' || first == '*'), lastRegex && isWord(first):
				out.WriteByte(' ')
			}
		}
		pending = 0
		out.WriteString(string(tok))
		prev = tok[len(tok)-1]
		prior := last
		lastRegex = false
		afterBlock = false
		if string(tok) == "(" {
			controlParens = append(controlParens, prior == "if" || prior == "while" || prior == "for" || prior == "with" || prior == "switch" || prior == "catch")
		}
		afterControl = false
		if string(tok) == ")" && len(controlParens) > 0 {
			afterControl = controlParens[len(controlParens)-1]
			controlParens = controlParens[:len(controlParens)-1]
		}
		last = string(tok)
	}
	regexAllowed := func() bool {
		if lastRegex {
			return false
		}
		if last == "" || afterControl || afterBlock {
			return true
		}
		r, _ := utf8.DecodeLastRuneInString(last)
		if isWord(r) {
			return regexAfterWord(last)
		}
		return !strings.ContainsRune(")]}\"'`", r)
	}
	// readTemplate scans from j (just after a backtick or the closing brace
	// of ${...}) and returns the index after the closing backtick or after "${".
	readTemplate := func(j, at int) (int, bool, error) {
		for j < n {
			c := s[j]
			if c == '\\' {
				j += 2
				continue
			}
			if c == '`' {
				return j + 1, false, nil
			}
			if c == '$' && j+1 < n && s[j+1] == '{' {
				return j + 2, true, nil
			}
			j++
		}
		return 0, false, fmt.Errorf("minify: unterminated template literal at offset %d", at)
	}

	for i := 0; i < n; {
		c := s[i]
		switch {
		case isJSSpace(c):
			j := i
			nl := false
			for j < n && isJSSpace(s[j]) {
				if jsNewline(s[j]) {
					nl = true
				}
				j++
			}
			if pending != '\n' {
				pending = ' '
				if nl {
					pending = '\n'
				}
			}
			i = j
		case c == '/' && i+1 < n && s[i+1] == '/':
			j := i
			for j < n && !jsNewline(s[j]) {
				j++
			}
			i = j
		case c == '/' && i+1 < n && s[i+1] == '*':
			j := i + 2
			nl := false
			for {
				if j+1 >= n {
					return "", fmt.Errorf("minify: unterminated comment at offset %d", i)
				}
				if s[j] == '*' && s[j+1] == '/' {
					break
				}
				j++
			}
			for _, r := range s[i:j] {
				if jsNewline(r) {
					nl = true
					break
				}
			}
			if nl {
				pending = '\n'
			} else if pending == 0 {
				pending = ' '
			}
			i = j + 2
		case c == '"' || c == '\'':
			j := i + 1
			for j < n && s[j] != c {
				switch s[j] {
				case '\\':
					j++
				case '\n':
					return "", fmt.Errorf("minify: unterminated string at offset %d", i)
				}
				j++
			}
			if j >= n {
				return "", fmt.Errorf("minify: unterminated string at offset %d", i)
			}
			emit(s[i : j+1])
			i = j + 1
		case c == '`' || (c == '}' && len(braces) > 0 && braces[len(braces)-1]):
			if c == '}' {
				braces = braces[:len(braces)-1]
			}
			j, opened, err := readTemplate(i+1, i)
			if err != nil {
				return "", err
			}
			emit(s[i:j])
			if opened {
				braces = append(braces, true)
			}
			i = j
		case c == '{':
			block := last == "" || last == ")" || last == ">" || last == "else" || last == "try" || last == "finally" || last == "do" || last == ";" || last == "{" || afterBlock || last == "static" || (last == ":" && (len(blocks) == 0 || blocks[len(blocks)-1]))
			if classPending {
				block = classStatement
				classPending = false
			}
			blocks = append(blocks, block)
			braces = append(braces, false)
			emit(s[i : i+1])
			i++
		case c == '}':
			block := false
			if len(blocks) > 0 {
				block = blocks[len(blocks)-1]
				blocks = blocks[:len(blocks)-1]
			}
			if len(braces) > 0 {
				braces = braces[:len(braces)-1]
			}
			emit(s[i : i+1])
			afterBlock = block
			i++
		case c == '/' && regexAllowed():
			j, inClass := i+1, false
		scan:
			for {
				if j >= n {
					return "", fmt.Errorf("minify: unterminated regular expression at offset %d", i)
				}
				d := s[j]
				switch {
				case d == '\\':
					j += 2
					continue
				case d == '[':
					inClass = true
				case d == ']':
					inClass = false
				case d == '/' && !inClass:
					break scan
				case jsNewline(d):
					return "", fmt.Errorf("minify: unterminated regular expression at offset %d", i)
				}
				j++
			}
			j++
			for j < n && unicode.IsLetter(s[j]) {
				j++
			}
			emit(s[i:j])
			lastRegex = true
			i = j
		case isWord(c):
			j := i
			for j < n && isWord(s[j]) {
				j++
			}
			if string(s[i:j]) == "class" {
				classPending = true
				classStatement = last == "" || last == ";" || last == "{" || afterBlock || last == "export" || last == "default"
			}
			emit(s[i:j])
			i = j
		default:
			emit(s[i : i+1])
			i++
		}
	}
	return strings.TrimFunc(out.String(), isJSSpace) + "\n", nil
}

// CSS minifies a stylesheet. It returns an error for an unterminated comment
// or string, or for input that is not valid UTF-8.
func CSS(src string) (string, error) {
	s, err := decode(src)
	if err != nil {
		return "", err
	}
	n := len(s)
	out := make([]rune, 0, n)
	for i := 0; i < n; {
		c := s[i]
		switch {
		case i+4 <= n && strings.EqualFold(string(s[i:i+4]), "url("):
			end, err := cssURLEnd(s, i+4)
			if err != nil {
				return "", err
			}
			out = append(out, s[i:end]...)
			i = end
		case c == '/' && i+1 < n && s[i+1] == '*':
			j := i + 2
			for j+1 < n && (s[j] != '*' || s[j+1] != '/') {
				j++
			}
			if j+1 >= n {
				return "", fmt.Errorf("minify: unterminated comment at offset %d", i)
			}
			out = append(out, ' ')
			i = j + 2
		case c == '"' || c == '\'':
			j := i + 1
			for j < n && s[j] != c {
				if s[j] == '\\' {
					j += 2
				} else {
					j++
				}
			}
			if j >= n {
				return "", fmt.Errorf("minify: unterminated string at offset %d", i)
			}
			out = append(out, s[i:j+1]...)
			i = j + 1
		default:
			out = append(out, c)
			i++
		}
	}

	var b strings.Builder
	var text []rune
	flush := func() {
		b.WriteString(cssText(text))
		text = text[:0]
	}
	for k := 0; k < len(out); {
		if k+4 <= len(out) && strings.EqualFold(string(out[k:k+4]), "url(") {
			end, err := cssURLEnd(out, k+4)
			if err != nil {
				return "", err
			}
			flush()
			b.WriteString(string(out[k:end]))
			k = end
			continue
		}
		if out[k] == '"' || out[k] == '\'' {
			if end, ok := matchCSSString(out, k); ok {
				flush()
				b.WriteString(string(out[k:end]))
				k = end
				continue
			}
		}
		text = append(text, out[k])
		k++
	}
	flush()
	return finish(b.String()), nil
}

// matchCSSString reports the end of a quoted string. Escaped line breaks are
// CSS line continuations and must remain inside the preserved string token.
func matchCSSString(s []rune, k int) (int, bool) {
	q := s[k]
	for m := k + 1; m < len(s); {
		switch s[m] {
		case q:
			return m + 1, true
		case '\\':
			if m+1 >= len(s) {
				return 0, false
			}
			m += 2
		default:
			m++
		}
	}
	return 0, false
}

// cssText minifies a CSS fragment that lies outside any string.
func cssText(p []rune) string {
	// Collapse whitespace runs to one space.
	c := make([]rune, 0, len(p))
	for i := 0; i < len(p); i++ {
		if isSpace(p[i]) {
			for i+1 < len(p) && isSpace(p[i+1]) {
				i++
			}
			c = append(c, ' ')
			continue
		}
		c = append(c, p[i])
	}
	// Drop one optional space on each side of { } ; , >.
	isPunct := func(r rune) bool { return r == '{' || r == '}' || r == ';' || r == ',' || r == '>' }
	d := make([]rune, 0, len(c))
	for i := 0; i < len(c); {
		switch {
		case c[i] == ' ' && i+1 < len(c) && isPunct(c[i+1]):
			d = append(d, c[i+1])
			i += 2
			if i < len(c) && c[i] == ' ' {
				i++
			}
		case isPunct(c[i]):
			d = append(d, c[i])
			i++
			if i < len(c) && c[i] == ' ' {
				i++
			}
		default:
			d = append(d, c[i])
			i++
		}
	}
	r := strings.ReplaceAll(string(d), ": ", ":")
	return strings.ReplaceAll(r, ";}", "}")
}

func cssURLEnd(s []rune, start int) (int, error) {
	quote := rune(0)
	for i := start; i < len(s); i++ {
		if s[i] == '\\' {
			i++
			continue
		}
		if quote != 0 {
			if s[i] == quote {
				quote = 0
			}
			continue
		}
		if s[i] == '\'' || s[i] == '"' {
			quote = s[i]
			continue
		}
		if s[i] == ')' {
			return i + 1, nil
		}
	}
	return 0, fmt.Errorf("minify: unterminated URL at offset %d", start)
}

func jsNewline(r rune) bool { return r == '\n' || r == '\r' || r == '\u2028' || r == '\u2029' }
