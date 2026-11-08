package minify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
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
	for _, s := range []string{"a + +b", "return /re/g", "`a${ `b${x}` }c`", "/* x\n */ a", "'s\\'x'", "x = a\n/ 2", "é \\u0061", "class A {}\n/a  b/", "class A extends B {}\n/a  b/", "lbl: {} /a  b/", "switch(x){case 1: {}/a  b/}", "class A {static {}} /a  b/", "\u202f/"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		if len(in) > 4096 {
			return
		}
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
		if len(in) > 4096 {
			return
		}
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

func TestURLAndControlRegex(t *testing.T) {
	for _, input := range []string{`a{background:url(data:image/svg+xml;base64,abc==)}`, `a{background:url(https://example.test/a//b.png)}`, `a{background:url(data:x;a;b)}`} {
		got, err := CSS(input)
		if err != nil || strings.TrimSpace(got) != input {
			t.Fatalf("CSS %q: %q %v", input, got, err)
		}
	}
	for _, input := range []string{`if (ready) /a[ /]b/.test(value);`, `while (next()) /https?:\/\//.test(value);`, `function f() { return (value) / 2; }`} {
		got, err := JS(input)
		if err != nil {
			t.Fatal(err)
		}
		if !nodeParses(t, got) {
			t.Fatalf("minification changed valid syntax: %q => %q", input, got)
		}
	}
}

func nodeParses(t testing.TB, input string) bool {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node syntax oracle requires node")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, "--check", "--input-type=commonjs")
	cmd.Stdin = strings.NewReader(input)
	err = cmd.Run()
	if ctx.Err() != nil {
		t.Skip("Node syntax oracle timed out")
	}
	return err == nil
}

func FuzzJSParseOracle(f *testing.F) {
	for _, input := range []string{`if (x) /a[ /]b/.test(y);`, `while (x()) /x/.test(y);`, `const x = (a) / 2;`, "const x = `hello ${1+2}`;"} {
		f.Add(input)
	}
	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 4096 {
			return
		}
		output, err := JS(input)
		if err == nil {
			checkLiteralOracle(t, [][2]string{{input, output}})
		}
	})
}

func TestBlockRegexLiteralsAndIdempotency(t *testing.T) {
	for _, input := range []string{`if (true) {} /a  b/.test("a  b");`, `function f() {} /a  b/g.test('a  b');`, `const f = () => {}; /a  b/.test("a  b");`, `const x = {} / 2;`, `/0/ return/ /`} {
		out, err := JS(input)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(input, `/a  b/`) && !strings.Contains(out, `/a  b/`) {
			t.Fatalf("regex literal changed: %s", out)
		}
		for _, literal := range []string{`"a  b"`, `'a  b'`} {
			if strings.Contains(input, literal) && !strings.Contains(out, literal) {
				t.Fatalf("string literal changed: %s", out)
			}
		}
		again, err := JS(out)
		if err != nil || again != out {
			t.Fatalf("not idempotent: %q => %q (%v)", out, again, err)
		}
	}
}

// Node's bundled parser provides a literal oracle independent of our tokenizer.
// Batch inputs keep the deterministic mutation corpus fast even without fuzz coverage.
func checkLiteralOracle(t testing.TB, pairs [][2]string) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("literal oracle requires Node")
	}
	program := `const fs=require('fs'),acorn=require('internal/deps/acorn/acorn/dist/acorn');
const pairs=JSON.parse(fs.readFileSync(0,'utf8'));const options={ecmaVersion:'latest'};
function literals(s){return [...acorn.tokenizer(s,options)].filter(t=>['string','regexp','template'].includes(t.type.label)).map(t=>s.slice(t.start,t.end))}
for(let i=0;i<pairs.length;i++){const [a,b]=pairs[i];try{acorn.parse(a,options)}catch{continue}try{acorn.parse(b,options)}catch{throw Error('output syntax changed at pair '+i)}if(JSON.stringify(literals(a))!==JSON.stringify(literals(b)))throw Error('literal changed at pair '+i)}`
	encoded, err := json.Marshal(pairs)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, "--expose-internals", "-e", program)
	cmd.Stdin = bytes.NewReader(encoded)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("literal oracle: %v: %s", err, output)
	}
}

func TestJSDeterministicLiteralMutations(t *testing.T) {
	var pairs [][2]string
	for i := 0; i < 1000; i++ {
		space := strings.Repeat(" ", i%9+1)
		input := fmt.Sprintf(`if (true) {%s} /a%sb[ /]c/g.test("a%sb"); const value={x:%d}/2; const text='escaped\\value';`, space, space, space, i)
		output, err := JS(input)
		if err != nil {
			t.Fatal(err)
		}
		again, err := JS(output)
		if err != nil || again != output {
			t.Fatalf("mutation %d not idempotent", i)
		}
		pairs = append(pairs, [2]string{input, output})
	}
	checkLiteralOracle(t, pairs)
}

func TestReportAssetLiteralParity(t *testing.T) {
	source, err := os.ReadFile("../../report/assets/app.js")
	if err != nil {
		t.Fatal(err)
	}
	output, err := JS(string(source))
	if err != nil {
		t.Fatal(err)
	}
	checkLiteralOracle(t, [][2]string{{string(source), output}})
}

func TestRound13BlockRegex(t *testing.T) {
	inputs := []string{"class A {}\n/a  b/", "class A extends B {}\n/a  b/", "switch(x){case 1: {}/a  b/.test(x)}", "lbl: {}\n/a  b/", "class A {static {}}\n/a  b/", "const C=class A {}; /a  b/"}
	var pairs [][2]string
	for _, input := range inputs {
		out, e := JS(input)
		if e != nil {
			t.Fatal(e)
		}
		pairs = append(pairs, [2]string{input, out})
	}
	checkLiteralOracle(t, pairs)
}

func TestCSSStringLineContinuation(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n", "\r"} {
		source := "a { content: \"before\\" + newline + "after\"; }"
		got, err := CSS(source)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, "\"before\\"+newline+"after\"") {
			t.Fatalf("string continuation changed: %q", got)
		}
		again, err := CSS(got)
		if err != nil || got != again {
			t.Fatalf("unstable: %q -> %q (%v)", got, again, err)
		}
	}
}
