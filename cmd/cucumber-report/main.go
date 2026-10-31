// Command cucumber-report renders a run directory's cucumber.json as an HTML report.
//
//	cucumber-report [-title T] [-name N] [-options options.json] [-o parent-dir] <run-dir>
//
// It writes a new <run-dir>/html-XXXXXX/index.html (or under -o) and prints only
// "HTML report: <path>". Errors go to standard error and never quote the results' content.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/qatoolist/cucumber-reporter/report"
)

const usage = "usage: cucumber-report [-title T] [-name N] [-options options.json] [-o parent-dir] <run-dir>\n"

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("cucumber-report", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	title := fs.String("title", "", "document title")
	name := fs.String("name", "", "heading shown on the page (default: the title)")
	options := fs.String("options", "", "JSON file with the report options (report.Options, \"version\": 2)")
	parent := fs.String("o", "", "directory to create html-XXXXXX in (default: the run directory)")
	if err := fs.Parse(args); err != nil || fs.NArg() != 1 {
		_, _ = fmt.Fprint(stderr, usage)
		return 2
	}
	runDir := fs.Arg(0)

	opt := report.Options{Version: report.OptionsVersion}
	if *options != "" {
		o, err := readOptions(*options)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "cucumber-report: -options: %v\n", err)
			return 1
		}
		opt = o
	}
	if *title != "" {
		opt.Title = *title
	}
	if *name != "" {
		opt.Name = *name
	}
	out := *parent
	if out == "" {
		out = runDir
	}
	dir, err := report.RenderDirTo(runDir, out, opt)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "cucumber-report: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "HTML report: %s\n", filepath.Join(dir, "index.html"))
	return 0
}

var (
	errOptionsVersion = errors.New("unsupported version; the options file needs \"version\": 2")
	errOptionsInvalid = errors.New("not a valid options file")
	errOptionsKind    = errors.New("the options file is a symbolic link or not a regular file")
	errOptionsRead    = errors.New("cannot read the options file")
)

// maxOptionsBytes bounds the -options file; annotations for a large run fit comfortably.
const maxOptionsBytes = 16 << 20

// readOptions reads the -options file: report.Options as JSON, decoded strictly (unknown keys and
// trailing data are refused). Its errors are fixed messages that never quote the file; the renderer
// validates the values.
func readOptions(path string) (report.Options, error) {
	var opt report.Options
	path = filepath.Clean(path)
	info, err := os.Lstat(path)
	if err != nil {
		return opt, errOptionsRead
	}
	if !info.Mode().IsRegular() {
		return opt, errOptionsKind
	}
	f, err := os.Open(path)
	if err != nil {
		return opt, errOptionsRead
	}
	defer func() { _ = f.Close() }() // read only
	if opened, statErr := f.Stat(); statErr != nil || !os.SameFile(info, opened) {
		return opt, errOptionsKind
	}
	data, err := io.ReadAll(io.LimitReader(f, maxOptionsBytes+1))
	if err != nil {
		return opt, errOptionsRead
	}
	if len(data) > maxOptionsBytes {
		return opt, errOptionsInvalid
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&opt); err != nil {
		return opt, errOptionsInvalid
	}
	if _, err := dec.Token(); err != io.EOF {
		return opt, errOptionsInvalid
	}
	if opt.Version != report.OptionsVersion {
		return opt, errOptionsVersion
	}
	return opt, nil
}
