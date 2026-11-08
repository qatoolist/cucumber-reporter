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
	"strings"

	"github.com/qatoolist/cucumber-reporter/internal/textsafe"
	"github.com/qatoolist/cucumber-reporter/report"
)

const usage = "usage: cucumber-report [-title T] [-name N] [-options options.json] [-o parent-dir] <run-dir>\n" +
	"       cucumber-report -enhanced [-o parent-dir] [-options options.json] [-fail-on-unexpected] [-run-config run.json] [-baseline summary.json] [-history history.json] [-bundle] [-format json|messages] <input> [<input> ...]\n" +
	"       cucumber-report -validate [-enhanced] [-strict] [-options options.json] [-run-config run.json] [-baseline summary.json] [-history history.json] [-fail-on-unexpected] [-format json|messages] <input> [<input> ...]\n" +
	"       cucumber-report -version\n" +
	"       cucumber-report compress-pdf [-quality high|balanced|small] [-o output.pdf] [-timeout 5m] input.pdf\n"

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	stderr = diagnosticWriter{stderr}
	if len(args) > 0 && args[0] == "compress-pdf" {
		return runCompressPDF(args[1:], stdout, stderr)
	}
	if len(args) == 1 && args[0] == "-version" {
		_, _ = fmt.Fprintln(stdout, "cucumber-report "+version())
		return 0
	}
	fs := flag.NewFlagSet("cucumber-report", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	title := fs.String("title", "", "document title")
	name := fs.String("name", "", "heading shown on the page (default: the title)")
	options := fs.String("options", "", "JSON file with the report options (report.Options, \"version\": 2)")
	parent := fs.String("o", "", "directory to create html-XXXXXX in (default: the run directory)")
	advanced := advancedFlags{}
	fs.StringVar(&advanced.config, "run-config", "", "optional run configuration")
	fs.StringVar(&advanced.baseline, "baseline", "", "optional summary.json baseline")
	fs.StringVar(&advanced.history, "history", "", "optional history.json")
	fs.StringVar(&advanced.format, "format", "json", "json or messages")
	fs.BoolVar(&advanced.validate, "validate", false, "validate without writing")
	fs.BoolVar(&advanced.strict, "strict", false, "reject unsupported execution fields")
	fs.BoolVar(&advanced.bundle, "bundle", false, "preserve all evidence and create ZIP")
	fs.BoolVar(&advanced.fail, "fail-on-unexpected", false, "exit 3 after writing on unexpected failures")
	enhanced := fs.Bool("enhanced", false, "write run analysis and machine-readable exports")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, usage)
			return 0
		}
		fmt.Fprintf(stderr, "cucumber-report: %v\n%s", err, usage)
		return 2
	}
	for _, arg := range fs.Args() {
		if strings.HasPrefix(arg, "-") {
			fmt.Fprintln(stderr, "cucumber-report: flags must come before inputs")
			return 2
		}
	}
	if advanced.strict && !advanced.validate {
		fmt.Fprintln(stderr, "cucumber-report: -strict requires -validate")
		return 2
	}
	if fs.NArg() > 1 && !*enhanced && !advanced.validate {
		fmt.Fprintln(stderr, "cucumber-report: plain mode takes one directory; use -enhanced for multiple inputs")
		return 2
	}
	if fs.NArg() < 1 {
		_, _ = fmt.Fprint(stderr, usage)
		return 2
	}
	advanced.format = strings.ToLower(advanced.format)
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
	plainValidation := advanced.validate && !*enhanced && fs.NArg() == 1 && advanced.config == "" && advanced.baseline == "" && advanced.history == "" && !advanced.bundle && !advanced.fail && advanced.format == "json"
	if plainValidation {
		return validatePlain(runDir, opt, advanced.strict, stdout, stderr)
	}
	if *enhanced || fs.NArg() > 1 || advanced.config != "" || advanced.baseline != "" || advanced.history != "" || advanced.validate || advanced.strict || advanced.bundle || advanced.fail || advanced.format != "json" {
		opt.BundleAttachments = opt.BundleAttachments || advanced.bundle
		return runAdvanced(fs.Args(), *parent, opt, advanced, stdout, stderr)
	}
	out := *parent
	if out == "" {
		out = runDir
	}
	dir, err := report.RenderDirTo(runDir, out, opt)
	if err != nil {
		if info, statErr := os.Lstat(runDir); errors.Is(err, report.ErrNoInput) && statErr == nil && info.IsDir() {
			err = fmt.Errorf("%w; for nested shard directories, use -enhanced and list each shard as a separate input", err)
		}
		_, _ = fmt.Fprintf(stderr, "cucumber-report: %v\n", err)
		return 1
	}
	warnOutputSize(dir, stderr)
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
	var keys map[string]json.RawMessage
	if err = json.Unmarshal(data, &keys); err != nil {
		return opt, errOptionsInvalid
	}
	for key := range keys {
		if strings.EqualFold(key, "bundleAttachments") {
			return opt, errors.New("bundleAttachments is Go-only; use -enhanced -bundle")
		}
		if strings.EqualFold(key, "analysis") {
			return opt, errOptionsInvalid
		}
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

func warnOutputSize(dir string, stderr io.Writer) {
	if info, err := os.Stat(filepath.Join(dir, "index.html")); err == nil && info.Size() > 200<<20 {
		_, _ = fmt.Fprintf(stderr, "Notice: index.html is %.1f MiB; opening a report this large may be slow. Consider separate shard reports for browsing.\n", float64(info.Size())/(1<<20))
	}
}

// Diagnostic output never emits terminal controls supplied by names or paths.
type diagnosticWriter struct{ io.Writer }

func (w diagnosticWriter) Write(p []byte) (int, error) {
	_, err := io.WriteString(w.Writer, textsafe.Display(string(p)))
	if err != nil {
		return 0, err
	}
	return len(p), nil
}
