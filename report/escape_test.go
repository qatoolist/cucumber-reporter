package report

import (
	"encoding/json"
	"html"
	"strings"
	"testing"
	"unicode"

	"github.com/qatoolist/cucumber-reporter/cucumber"
)

// A minimal tokenizer for the renderer's own output, written for these tests so the module needs no
// HTML parser. html/template always quotes attribute values and escapes '<' in text, so any element
// that data managed to inject shows up here as an extra tag or attribute.

type attr struct{ name, value string }

type token struct {
	kind  byte // 't' text, 's' start tag, 'e' end tag, 'c' comment or doctype, 'r' raw script/style text
	name  string
	attrs []attr
	text  string
}

func tokenize(t testing.TB, doc string) []token {
	t.Helper()
	var out []token
	i := 0
	for i < len(doc) {
		if doc[i] != '<' {
			j := strings.IndexByte(doc[i:], '<')
			if j < 0 {
				j = len(doc) - i
			}
			out = append(out, token{kind: 't', text: doc[i : i+j]})
			i += j
			continue
		}
		switch {
		case strings.HasPrefix(doc[i:], "<!--"):
			j := strings.Index(doc[i:], "-->")
			if j < 0 {
				t.Fatalf("unterminated comment at %d", i)
			}
			out = append(out, token{kind: 'c'})
			i += j + 3
		case strings.HasPrefix(doc[i:], "<!"):
			j := strings.IndexByte(doc[i:], '>')
			out = append(out, token{kind: 'c'})
			i += j + 1
		case strings.HasPrefix(doc[i:], "</"):
			j := strings.IndexByte(doc[i:], '>')
			if j < 0 {
				t.Fatalf("unterminated end tag at %d", i)
			}
			out = append(out, token{kind: 'e', name: strings.ToLower(strings.TrimSpace(doc[i+2 : i+j]))})
			i += j + 1
		default:
			tok, n := startTag(t, doc[i:])
			out = append(out, tok)
			i += n
			if tok.name == "script" || tok.name == "style" {
				j := strings.Index(strings.ToLower(doc[i:]), "</"+tok.name)
				if j < 0 {
					t.Fatalf("unterminated %s", tok.name)
				}
				out = append(out, token{kind: 'r', text: doc[i : i+j]})
				i += j
			}
		}
	}
	return out
}

func startTag(t testing.TB, s string) (token, int) {
	i := 1
	for i < len(s) && isNameByte(s[i]) {
		i++
	}
	if i == 1 {
		t.Fatalf("'<' not followed by a tag name: %.40q", s)
	}
	tok := token{kind: 's', name: strings.ToLower(s[1:i])}
	for {
		for i < len(s) && unicode.IsSpace(rune(s[i])) {
			i++
		}
		if i >= len(s) {
			t.Fatalf("unterminated tag %s", tok.name)
		}
		if s[i] == '>' {
			return tok, i + 1
		}
		if strings.HasPrefix(s[i:], "/>") {
			return tok, i + 2
		}
		start := i
		for i < len(s) && !unicode.IsSpace(rune(s[i])) && s[i] != '=' && s[i] != '>' && s[i] != '/' {
			i++
		}
		a := attr{name: strings.ToLower(s[start:i])}
		if i < len(s) && s[i] == '=' {
			i++
			if i < len(s) && (s[i] == '"' || s[i] == '\'') {
				q := s[i]
				j := strings.IndexByte(s[i+1:], q)
				if j < 0 {
					t.Fatalf("unterminated attribute value in %s", tok.name)
				}
				a.value = html.UnescapeString(s[i+1 : i+1+j])
				i += j + 2
			} else {
				vs := i
				for i < len(s) && !unicode.IsSpace(rune(s[i])) && s[i] != '>' {
					i++
				}
				a.value = s[vs:i]
			}
		}
		if a.name == "" {
			t.Fatalf("empty attribute name in %s", tok.name)
		}
		tok.attrs = append(tok.attrs, a)
	}
}

func isNameByte(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '-'
}

// shape is the document's markup skeleton: every tag and attribute name, without text or values.
func shape(tokens []token) []string {
	var out []string
	for _, tk := range tokens {
		switch tk.kind {
		case 's':
			names := make([]string, len(tk.attrs))
			for i, a := range tk.attrs {
				names[i] = a.name
			}
			out = append(out, "<"+tk.name+" "+strings.Join(names, " ")+">")
		case 'e':
			out = append(out, "</"+tk.name+">")
		}
	}
	return out
}

func allText(tokens []token) string {
	var b strings.Builder
	for _, tk := range tokens {
		if tk.kind == 't' {
			b.WriteString(html.UnescapeString(tk.text))
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// checkSafe asserts the invariants every render must keep, whatever the data.
func checkSafe(t testing.TB, tokens []token) {
	t.Helper()
	scripts, styles := 0, 0
	for _, tk := range tokens {
		if tk.kind != 's' {
			continue
		}
		switch tk.name {
		case "script":
			scripts++
		case "style":
			styles++
		case "iframe", "object", "embed", "img", "image", "link", "base", "form", "foreignobject", "use", "animate", "set":
			t.Errorf("unexpected element <%s>", tk.name)
		}
		for _, a := range tk.attrs {
			switch {
			case strings.HasPrefix(a.name, "on"):
				t.Errorf("event handler attribute %s on <%s>", a.name, tk.name)
			case a.name == "style":
				t.Errorf("style attribute on <%s>", tk.name)
			case a.name == "href" || a.name == "src" || a.name == "xlink:href" || a.name == "action" || a.name == "formaction":
				if a.name != "href" || (!strings.HasPrefix(a.value, "#") && !strings.HasPrefix(a.value, "attachment-")) {
					t.Errorf("<%s %s=%q> is not an in-page anchor or renderer-chosen sidecar", tk.name, a.name, a.value)
				}
			}
		}
	}
	if scripts != 1 || styles != 1 {
		t.Errorf("scripts = %d, styles = %d; want 1 and 1", scripts, styles)
	}
}

// display is how a data string must read back from the page: html/template turns NUL into U+FFFD and
// the renderer replaces invalid UTF-8 with U+FFFD.
func display(s string) string {
	return strings.ReplaceAll(strings.ToValidUTF8(s, "�"), "\x00", "�")
}

var hostile = []string{
	`<script>alert(1)</script>`,
	`"><img src=x onerror=alert(1)>`,
	`'><svg onload=alert(1)>`,
	`javascript:alert(document.domain)`,
	`</style><script>alert(1)</script><style>`,
	`</script><script>alert(1)</script>`,
	`</title></textarea></pre><b>x</b>`,
	`<!-- comment --><![CDATA[x]]>`,
	`<a href="https://attacker.example/">link</a>`,
	`data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==`,
	`url(javascript:alert(1)) expression(alert(1)) } body { background:red }`,
	"line separator paragraph",
	"invalid \xff\xfe utf-8 \xc3",
	"nul\x00byte",
	`{{.}} {{template "x"}} ${x} #{x}`,
	`&lt;not decoded&gt; &amp; &#60;`,
	`<svg><foreignObject><iframe src="x"></iframe></foreignObject></svg>`,
	`<math><mi xlink:href="javascript:x">y</mi></math>`,
	"\"; alert(1); //",
}

// hostileRun puts s into every data field the renderer shows.
func hostileRun(s string) ([]cucumber.Feature, Options) {
	jsonDoc, _ := json.Marshal(map[string]string{s: s})
	// Attachment bodies must be valid UTF-8 to be shown as text; invalid bodies are shown as binary,
	// which TestAttachments covers.
	body := strings.ToValidUTF8(s, "\uFFFD")
	tag := []cucumber.Tag{{Name: s, Line: 1}}
	features := []cucumber.Feature{{
		URI: s, ID: s, Keyword: s, Name: s, Description: s, Line: 1, Tags: tag,
		Elements: []cucumber.Scenario{{
			ID: s, Keyword: s, Name: s, Description: s, Type: s, Line: 2, Tags: tag,
			Steps: []cucumber.Step{
				{
					Keyword: s, Name: s, Line: 3,
					DocString:  &cucumber.DocString{Value: s, ContentType: s, Line: 4},
					Rows:       []cucumber.Row{{Cells: []string{s, s}}, {Cells: []string{s, s}}},
					Match:      cucumber.Match{Location: s},
					Result:     cucumber.Result{Status: cucumber.StatusFailed, Error: s},
					Embeddings: []cucumber.Embedding{{Name: s, MimeType: "text/plain", Data: b64(body)}, {Name: s, MimeType: "text/html", Data: b64(body)}, {Name: s, MimeType: "application/json", Data: b64(string(jsonDoc))}, {Name: s, MimeType: "application/octet-stream", Data: b64(s)}},
				},
			},
		}},
	}}
	opt := Options{
		Version: 1, Title: s, Name: s,
		RunFacts: []Fact{{Label: s, Value: s}},
		Sections: []Section{{
			ID: "hostile", Title: s, Tone: ToneFail, Summary: []Fact{{Label: s, Value: s}}, Rows: []Fact{{Label: s, Value: s}}, Note: s,
			Chart: &ChartSeries{Title: s, Segments: []Segment{{s, 1, ToneFail}, {s, 2, ToneOK}}},
		}},
	}
	return features, opt
}

func renderHostile(t testing.TB, s string) string {
	t.Helper()
	features, opt := hostileRun(s)
	return mustRender(t, features, opt)
}

// TestEscaping renders each hostile string in every field and checks that the markup skeleton is
// identical to a render of a harmless string, that nothing active appears, and that the text reads
// back unchanged.
func TestEscaping(t *testing.T) {
	baseline := shape(tokenize(t, renderHostile(t, "placeholder")))
	for _, s := range hostile {
		t.Run(strings.ToValidUTF8(s, "?"), func(t *testing.T) {
			features, opt := hostileRun(s)
			doc := mustRender(t, features, opt)
			tokens := tokenize(t, doc)
			checkSafe(t, tokens)
			if got := shape(tokens); strings.Join(got, "\n") != strings.Join(baseline, "\n") {
				t.Errorf("markup skeleton changed with data %q", s)
			}
			if !strings.Contains(allText(tokens), display(s)) {
				t.Errorf("data %q does not read back as text", s)
			}
		})
	}
}

// TestHostileMimeType: the MIME type is data too; an unknown type shows name, type and size only.
func TestHostileMimeType(t *testing.T) {
	for _, s := range hostile {
		features := one(step("s", cucumber.StatusPassed, 1, cucumber.Embedding{Name: "a", MimeType: s, Data: b64("<b>content</b>")}))
		doc := mustRender(t, features, Options{Version: 1})
		tokens := tokenize(t, doc)
		checkSafe(t, tokens)
		text := allText(tokens)
		if strings.HasPrefix(strings.ToLower(s), "text/") {
			continue
		}
		if strings.Contains(text, "<b>content</b>") {
			t.Errorf("content of an attachment typed %q was shown", s)
		}
		if !strings.Contains(text, display(s)) {
			t.Errorf("MIME type %q does not read back as text", s)
		}
	}
}
