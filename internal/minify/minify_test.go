package minify

import (
	"strings"
	"testing"
)

type minifyFunc func(string) (string, error)

func runTable(t *testing.T, fn minifyFunc, cases []struct{ name, in, want string }) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := fn(tc.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			again, err := fn(got)
			if err != nil {
				t.Fatalf("second pass error: %v", err)
			}
			if again != got {
				t.Fatalf("not idempotent: %q then %q", got, again)
			}
		})
	}
}

func TestJS(t *testing.T) {
	runTable(t, JS, []struct{ name, in, want string }{
		{"line comment", "var a = 1; // c\nvar b = 2;", "var a=1;\nvar b=2;\n"},
		{"block comment inline", "a /* x */ b", "a b\n"},
		{"block comment with newline", "a /* x\n y */ b", "a\nb\n"},
		{"two block comments", "a /* c */ /* d */ b", "a b\n"},
		{"plus space plus", "a + +b", "a+ +b\n"},
		{"minus space minus", "a - -b", "a- -b\n"},
		{"plus minus", "a + -b", "a+-b\n"},
		{"postfix then unary", "a ++ +b", "a++ +b\n"},
		{"postfix plus", "a++ + b", "a++ +b\n"},
		{"regex after return", "return /re/g.test(x)", "return/re/g.test(x)\n"},
		{"regex after paren", "f( /a b/ )", "f(/a b/)\n"},
		{"regex after assign", "x = /[/]/.test(y)", "x=/[/]/.test(y)\n"},
		{"regex after typeof", "typeof /x/", "typeof/x/\n"},
		{"division after identifier", "a / b / c", "a/b/c\n"},
		{"division after paren", "(a) / 2", "(a)/2\n"},
		{"division after number", "x = a / 2 / 3", "x=a/2/3\n"},
		{"newline kept before division", "x = a\n\n  / 2", "x=a\n/2\n"},
		{"nested template", "`a${ `b${x}` }c`", "`a${`b${x}`}c`\n"},
		{"template with object literal", "`t ${ {a:1}.a } u`", "`t ${{a:1}.a} u`\n"},
		{"template escaped backtick", "x = `a\\`b`", "x=`a\\`b`\n"},
		{"string escaped double quote", "var s = \"a\\\"b\" ;", "var s=\"a\\\"b\";\n"},
		{"string escaped single quote", "var s = 'it\\'s' ;", "var s='it\\'s';\n"},
		{"string interior untouched", "s = \"a  b\"   +   'c  d'", "s=\"a  b\"+'c  d'\n"},
		{"word space", "let   x\t=\t1", "let x=1\n"},
		{"asi newlines collapse", "a\n\n\nb", "a\nb\n"},
		{"crlf is newline", "a\r\nb", "a\nb\n"},
		{"non-ascii identifiers", "var é = 1; var ü = é", "var é=1;var ü=é\n"},
		{"backslash is word", "\\u0061 b", "\\u0061 b\n"},
		{"division then regex literal", "a / /b/.x", "a/ /b/.x\n"},
		{"braces", "if (a) { b } else { c }", "if(a){b}else{c}\n"},
		{"empty", "", "\n"},
		{"whitespace only", "   \n  ", "\n"},
	})
}

func TestCSS(t *testing.T) {
	runTable(t, CSS, []struct{ name, in, want string }{
		{"descendant pseudo keeps space", "a :hover { color : red ; }", "a :hover{color :red}\n"},
		{"combinator and comma", "a  >  b , c { margin : 0 0 }", "a>b,c{margin :0 0}\n"},
		{"calc keeps operators", "calc(1px + 2px)", "calc(1px + 2px)\n"},
		{"comment removed", "/* c */ a{b:c;}", "a{b:c}\n"},
		{"string untouched", "a::before{content : \"x  ;  }\" ;}", "a::before{content :\"x  ;  }\"}\n"},
		{"escaped quote in string", "a{b:'it\\'s  x' ; c : d}", "a{b:'it\\'s  x';c :d}\n"},
		{"empty rule", "a{;}", "a{}\n"},
		{"at-rule", "@media (min-width: 1px) { a { b: c; } }", "@media (min-width:1px){a{b:c}}\n"},
		{"string value", "a { x : \"s\" ; }", "a{x :\"s\"}\n"},
		{"rules joined", "a{b:c ; }\n\nd{e:f;}", "a{b:c}d{e:f}\n"},
		{"whitespace only", "  \n\n ", "\n"},
	})
}

// The reference minifier removes ";}" in a single pass, so ";;}" shrinks by one
// semicolon per run. The port keeps that behavior for byte parity.
func TestCSSDoubleSemicolonMatchesReference(t *testing.T) {
	got, err := CSS("a { b: c; ; }")
	if err != nil || got != "a{b:c;}\n" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestErrors(t *testing.T) {
	const secret = "SECRETTEXT"
	js := map[string]string{
		"unterminated string":        "var s = '" + secret,
		"newline in string":          "var s = '" + secret + "\nx'",
		"unterminated template":      "`" + secret,
		"unterminated template expr": "`a${x}" + secret,
		"unterminated comment":       "a /* " + secret,
		"unterminated regex":         "x = /" + secret,
		"newline in regex":           "x = /" + secret + "\n/",
		"invalid utf8":               "a\xff" + secret,
	}
	for name, in := range js {
		t.Run("js "+name, func(t *testing.T) {
			got, err := JS(in)
			if err == nil {
				t.Fatalf("want error, got %q", got)
			}
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("error echoes input: %v", err)
			}
		})
	}
	css := map[string]string{
		"unterminated comment": "a{} /* " + secret,
		"unterminated string":  "a{b:'" + secret,
		"invalid utf8":         "a{b:\xff" + secret + "}",
	}
	for name, in := range css {
		t.Run("css "+name, func(t *testing.T) {
			got, err := CSS(in)
			if err == nil {
				t.Fatalf("want error, got %q", got)
			}
			if strings.Contains(err.Error(), secret) {
				t.Fatalf("error echoes input: %v", err)
			}
		})
	}
}

func FuzzJS(f *testing.F) {
	for _, s := range []string{"a + +b", "return /re/g", "`a${ `b${x}` }c`", "/* x\n */ a", "'s\\'x'", "x = a\n/ 2", "é \\u0061"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		out, err := JS(in)
		if err != nil {
			return
		}
		again, err := JS(out)
		if err != nil {
			t.Fatalf("second pass failed on %q (from %q): %v", out, in, err)
		}
		if again != out {
			t.Fatalf("not idempotent: %q -> %q -> %q", in, out, again)
		}
	})
}

func FuzzCSS(f *testing.F) {
	for _, s := range []string{"a :hover { b : c ; }", "calc(1px + 2px)", "a{b:'x ; y'}", "/* c */ a>b,c{;}", "a { b: \"s\" ; }"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		out, err := CSS(in)
		if err != nil {
			return
		}
		again, err := CSS(out)
		if err != nil {
			t.Fatalf("second pass failed on %q (from %q): %v", out, in, err)
		}
		// ";;}" loses one semicolon per pass in the reference algorithm; see
		// TestCSSDoubleSemicolonMatchesReference.
		if again != out && !strings.Contains(out, ";}") {
			t.Fatalf("not idempotent: %q -> %q -> %q", in, out, again)
		}
	})
}
