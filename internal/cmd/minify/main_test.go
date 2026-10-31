package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunMinifiesAndIsNoopWhenCurrent(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.js")
	dst := filepath.Join(dir, "a.min.js")
	if err := os.WriteFile(src, []byte("var a = 1; // c\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if code := run([]string{src, dst}, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "var a=1;\n" {
		t.Fatalf("got %q", got)
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("mode %v", info.Mode().Perm())
	}
	// Up-to-date destination must not be rewritten: make it read-only; a rewrite would fail.
	if err := os.Chmod(dst, 0o444); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	if code := run([]string{src, dst}, &stderr); code != 0 {
		t.Fatalf("second run exit %d: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr not empty: %s", stderr.String())
	}
}

func TestRunCSS(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.css")
	dst := filepath.Join(dir, "out.css")
	if err := os.WriteFile(src, []byte("a { b : c ; }"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if code := run([]string{src, dst}, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	got, _ := os.ReadFile(dst)
	if string(got) != "a{b :c}\n" {
		t.Fatalf("got %q", got)
	}
}

func TestRunUsageErrors(t *testing.T) {
	dir := t.TempDir()
	txt := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(txt, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, args := range map[string][]string{
		"bad extension": {txt, filepath.Join(dir, "o")},
		"no args":       {},
		"one arg":       {txt},
	} {
		t.Run(name, func(t *testing.T) {
			var stderr bytes.Buffer
			if code := run(args, &stderr); code != 2 {
				t.Fatalf("exit %d, want 2", code)
			}
			if !strings.Contains(stderr.String(), "usage: minify") {
				t.Fatalf("stderr %q lacks usage", stderr.String())
			}
		})
	}
}

func TestRunSyntaxErrorDoesNotEchoContent(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "bad.js")
	if err := os.WriteFile(src, []byte("var s = 'SECRETTEXT"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if code := run([]string{src, filepath.Join(dir, "o.js")}, &stderr); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if strings.Contains(stderr.String(), "SECRETTEXT") {
		t.Fatalf("stderr echoes content: %s", stderr.String())
	}
}
