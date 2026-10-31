// Command cucumber-report renders a run directory's cucumber.json as an HTML report.
//
//	cucumber-report [-title T] [-name N] [-defaults settings.json] [-o parent-dir] <run-dir>
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

const usage = "usage: cucumber-report [-title T] [-name N] [-defaults settings.json] [-o parent-dir] <run-dir>\n"

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("cucumber-report", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	title := fs.String("title", "", "document title")
	name := fs.String("name", "", "heading shown on the page (default: the title)")
	defaults := fs.String("defaults", "", "JSON settings file with the report's view defaults")
	parent := fs.String("o", "", "directory to create html-XXXXXX in (default: the run directory)")
	if err := fs.Parse(args); err != nil || fs.NArg() != 1 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	runDir := fs.Arg(0)

	opt := report.Options{Version: report.OptionsVersion, Title: *title, Name: *name}
	if *defaults != "" {
		d, err := readDefaults(*defaults)
		if err != nil {
			fmt.Fprintf(stderr, "cucumber-report: -defaults: %v\n", err)
			return 1
		}
		opt.Defaults = d
	}
	out := *parent
	if out == "" {
		out = runDir
	}
	dir, err := report.RenderDirTo(runDir, out, opt)
	if err != nil {
		fmt.Fprintf(stderr, "cucumber-report: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "HTML report: %s\n", filepath.Join(dir, "index.html"))
	return 0
}

// settingsFile is the -defaults format. pageSize is 25, 50, 100 or "all".
type settingsFile struct {
	Version  int                     `json:"version"`
	Charts   map[report.ChartID]bool `json:"charts"`
	Theme    string                  `json:"theme"`
	Density  string                  `json:"density"`
	Expand   string                  `json:"expand"`
	PageSize json.RawMessage         `json:"pageSize"`
}

var (
	errSettingsVersion = errors.New("unsupported version; the settings file needs \"version\": 1")
	errSettingsInvalid = errors.New("not a valid settings file")
	errSettingsKind    = errors.New("the settings file is a symbolic link or not a regular file")
	errSettingsRead    = errors.New("cannot read the settings file")
)

const maxSettingsBytes = 64 << 10

// readDefaults reads the -defaults file. Its errors are fixed messages that never quote the file.
func readDefaults(path string) (report.ViewDefaults, error) {
	var d report.ViewDefaults
	info, err := os.Lstat(path)
	if err != nil {
		return d, errSettingsRead
	}
	if !info.Mode().IsRegular() {
		return d, errSettingsKind
	}
	f, err := os.Open(path)
	if err != nil {
		return d, errSettingsRead
	}
	defer f.Close()
	if opened, err := f.Stat(); err != nil || !os.SameFile(info, opened) {
		return d, errSettingsKind
	}
	data, err := io.ReadAll(io.LimitReader(f, maxSettingsBytes+1))
	if err != nil {
		return d, errSettingsRead
	}
	if len(data) > maxSettingsBytes {
		return d, errSettingsInvalid
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var s settingsFile
	if err := dec.Decode(&s); err != nil {
		return d, errSettingsInvalid
	}
	if _, err := dec.Token(); err != io.EOF {
		return d, errSettingsInvalid
	}
	if s.Version != report.OptionsVersion {
		return d, errSettingsVersion
	}
	d = report.ViewDefaults{Charts: s.Charts, Theme: s.Theme, Density: s.Density, Expand: s.Expand}
	if len(s.PageSize) > 0 {
		var all string
		if err := json.Unmarshal(s.PageSize, &all); err == nil {
			if all != "all" {
				return d, errSettingsInvalid
			}
			d.PageSize = report.PageSizeAll
		} else if err := json.Unmarshal(s.PageSize, &d.PageSize); err != nil || d.PageSize <= 0 {
			return d, errSettingsInvalid
		}
	}
	return d, nil
}
