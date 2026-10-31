// Command minify minifies a JavaScript or CSS file for embedding in the report.
//
// Usage: minify <src> <dst>
//
// The source language is chosen by the src extension (.js or .css). dst is
// written with mode 0644 only when its contents would change, so running it
// from go generate is a no-op when the output is already up to date. Nothing
// is printed on success; errors go to stderr and never echo file content.
package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/qatoolist/cucumber-reporter/internal/minify"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	if len(args) != 2 {
		return usage(stderr)
	}
	var fn func(string) (string, error)
	switch filepath.Ext(args[0]) {
	case ".js":
		fn = minify.JS
	case ".css":
		fn = minify.CSS
	default:
		return usage(stderr)
	}
	src, err := os.ReadFile(args[0]) //nolint:gosec // paths are the operator's command-line arguments
	if err != nil {
		fmt.Fprintf(stderr, "minify: read %s: %v\n", args[0], err)
		return 1
	}
	out, err := fn(string(src))
	if err != nil {
		fmt.Fprintf(stderr, "minify: %s: %v\n", args[0], err)
		return 1
	}
	cur, err := os.ReadFile(args[1]) //nolint:gosec // see above
	if err == nil && bytes.Equal(cur, []byte(out)) {
		return 0
	}
	if err := os.WriteFile(args[1], []byte(out), 0o644); err != nil { //nolint:gosec // generated public asset, 0644 by design
		fmt.Fprintf(stderr, "minify: write %s: %v\n", args[1], err)
		return 1
	}
	if err := os.Chmod(args[1], 0o644); err != nil { //nolint:gosec // see above
		fmt.Fprintf(stderr, "minify: chmod %s: %v\n", args[1], err)
		return 1
	}
	return 0
}

func usage(stderr io.Writer) int {
	fmt.Fprintln(stderr, "usage: minify <src.js|src.css> <dst>")
	return 2
}
