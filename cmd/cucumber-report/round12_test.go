package main

import (
	"strings"
	"testing"
)

func TestCanonicalPseudoVersions(t *testing.T) {
	for _, input := range []string{"v0.2.1-0.20261107000000-abcdef123456", "v0.0.0-20261107000000-abcdef123456", "v0.3.0-pre.0.20261107000000-abcdef123456", "v0.2.1-0.20261107000000-abcdef123456+dirty"} {
		if got := versionFromModule(input); got != strings.TrimSuffix(developmentVersion, "-dev")+"-dev (abcdef123456)" {
			t.Fatalf("%s: %s", input, got)
		}
	}
}

func TestCLIInputHints(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{{[]string{"-strict", "somewhere"}, "-strict requires -validate"}, {[]string{"first", "second"}, "use -enhanced"}, {[]string{"-enhanced", "first", "-o"}, "flags must come before inputs"}} {
		_, stderr, code := command(t, tc.args...)
		if code != 2 || !strings.Contains(stderr, tc.want) {
			t.Fatalf("%v: %d %s", tc.args, code, stderr)
		}
	}
}

func TestPDFUsageDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"compress-pdf", "--quality", "bogus", "input.pdf"}, "quality"},
		{[]string{"compress-pdf", "--timeout", "0s", "input.pdf"}, "timeout"},
		{[]string{"compress-pdf", "input.pdf", "--o"}, "needs an argument"},
		{[]string{"compress-pdf", "one.pdf", "two.pdf"}, "one input"},
	} {
		_, stderr, code := command(t, tc.args...)
		if code != 2 || !strings.Contains(stderr, tc.want) {
			t.Fatalf("%v: %d %s", tc.args, code, stderr)
		}
	}
}
