package textsafe

import "testing"

func TestDisplay(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"\x1b[31mboom\x1b[0m", "boom"},
		{"nul\x00byte", `nul\u0000byte`},
		{"a\u202eb", `a\u202eb`},
		{"\x1b]0;title\x07", `\u001b]0;title\u0007`},
		{"line\ncol\tvalue", "line\ncol\tvalue"},
		{"bad\xff", "bad�"},
	} {
		if got := Display(tt.in); got != tt.want {
			t.Errorf("%q: %q != %q", tt.in, got, tt.want)
		}
	}
}
