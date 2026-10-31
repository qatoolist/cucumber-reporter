// Package report renders parsed Cucumber results as one self-contained, offline HTML document.
//
// All report data passes through html/template, which escapes it for its context. The page carries a
// Content-Security-Policy that admits only the embedded stylesheet and script by their SHA-256 hashes,
// so no markup, style or script can come from the data. Viewer preferences are stored in the
// browser's localStorage; generation-time defaults reach the page as data attributes.
package report

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"

	"github.com/qatoolist/cucumber-reporter/cucumber"
)

var (
	//go:embed assets/report.css
	styleSource string
	//go:embed assets/report.js
	scriptSource string
	//go:embed templates/*.gohtml
	templateFS embed.FS

	page = template.Must(template.New("page.gohtml").ParseFS(templateFS, "templates/*.gohtml"))

	// head and script are fixed at package initialisation from the embedded assets only. They are the
	// only template.HTML values in the package; report data never becomes template.HTML.
	head, script = assetElements()
)

func hashOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
}

func assetElements() (template.HTML, template.HTML) {
	if strings.Contains(strings.ToLower(styleSource), "</style") || strings.Contains(strings.ToLower(scriptSource), "</script") {
		panic("report: embedded asset closes its own element")
	}
	csp := "default-src 'none'; script-src " + hashOf(scriptSource) + "; style-src " + hashOf(styleSource) +
		"; img-src data:; base-uri 'none'; form-action 'none'; object-src 'none'"
	h := `<meta http-equiv="Content-Security-Policy" content="` + csp + `">` + "\n" +
		`<meta name="referrer" content="no-referrer">` + "\n" +
		"<style>" + styleSource + "</style>"
	return template.HTML(h), template.HTML("<script>" + scriptSource + "</script>")
}

// sidecarFunc stores an oversize text attachment beside the page under a renderer-chosen name.
type sidecarFunc func(name string, data []byte) error

// Render returns one HTML document for features. It is deterministic: the same input and options
// produce the same bytes. Text attachments larger than Options.Limits.InlineAttachment are left out
// with a notice; RenderDir writes them beside the page instead.
func Render(features []cucumber.Feature, opt Options) ([]byte, error) {
	return render(features, opt, nil)
}

func render(features []cucumber.Feature, opt Options, sidecar sidecarFunc) ([]byte, error) {
	opt, err := normalize(opt)
	if err != nil {
		return nil, err
	}
	if err := cucumber.Validate(features); err != nil {
		return nil, err
	}
	view, err := buildPage(features, opt, sidecar)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := page.Execute(&buf, view); err != nil {
		return nil, fmt.Errorf("report: execute template: %w", err)
	}
	if buf.Len() == 0 {
		return nil, ErrEmptyOutput
	}
	return buf.Bytes(), nil
}

// RenderDir reads <runDir>/cucumber.json and writes a new <runDir>/html-XXXXXX/index.html, returning
// the new directory. See RenderDirTo.
func RenderDir(runDir string, opt Options) (string, error) {
	return RenderDirTo(runDir, runDir, opt)
}

// RenderDirTo reads <runDir>/cucumber.json and writes a new <parentDir>/html-XXXXXX/index.html,
// returning the new directory. Neither runDir, parentDir nor cucumber.json may be a symbolic link,
// and cucumber.json must be a regular file. The report is written into a temporary directory inside
// parentDir (mode 0700, files 0600) and then renamed; earlier reports are never replaced, and a
// failed render leaves nothing behind. Oversize text attachments are written beside index.html as
// .txt or .json files and linked.
func RenderDirTo(runDir, parentDir string, opt Options) (string, error) {
	opt, err := normalize(opt)
	if err != nil {
		return "", err
	}
	if err := requireDir(runDir); err != nil {
		return "", err
	}
	if err := requireDir(parentDir); err != nil {
		return "", err
	}
	features, err := readResults(filepath.Join(runDir, "cucumber.json"), opt.Limits)
	if err != nil {
		return "", err
	}

	tmp, err := os.MkdirTemp(parentDir, ".html-report-")
	if err != nil {
		return "", fmt.Errorf("report: create output directory: %w", err)
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(tmp)
		}
	}()
	if err := os.Chmod(tmp, 0o700); err != nil {
		return "", fmt.Errorf("report: create output directory: %w", err)
	}
	doc, err := render(features, opt, func(name string, data []byte) error {
		return writeNew(filepath.Join(tmp, name), data)
	})
	if err != nil {
		return "", err
	}
	if err := writeNew(filepath.Join(tmp, "index.html"), doc); err != nil {
		return "", err
	}
	// The random suffix comes from MkdirTemp, so the final name is new unless another process created
	// it in the meantime; rename(2) cannot replace a non-empty directory either way.
	final := filepath.Join(parentDir, "html-"+strings.TrimPrefix(filepath.Base(tmp), ".html-report-"))
	if _, err := os.Lstat(final); !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("report: output directory %s already exists", final)
	}
	if err := os.Rename(tmp, final); err != nil {
		return "", fmt.Errorf("report: publish output directory: %w", err)
	}
	published = true
	return final, nil
}

func requireDir(dir string) error {
	info, err := os.Lstat(dir)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("%w: %s", ErrNoInput, dir)
	case err != nil:
		return fmt.Errorf("report: inspect %s: %w", dir, err)
	case !info.IsDir():
		return fmt.Errorf("%w: %s", ErrNotRegular, dir)
	}
	return nil
}

// readResults opens cucumber.json without following a symbolic link: it inspects the name with Lstat
// and checks that the opened file is that same regular file.
func readResults(path string, lim cucumber.Limits) ([]cucumber.Feature, error) {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil, fmt.Errorf("%w: %s", ErrNoInput, filepath.Dir(path))
	case err != nil:
		return nil, fmt.Errorf("report: inspect cucumber.json: %w", err)
	case !info.Mode().IsRegular():
		return nil, fmt.Errorf("%w: %s", ErrNotRegular, path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("report: open cucumber.json: %w", err)
	}
	defer func() { _ = f.Close() }() // read only
	opened, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("report: inspect cucumber.json: %w", err)
	}
	if !os.SameFile(info, opened) {
		return nil, fmt.Errorf("%w: %s changed while opening", ErrNotRegular, path)
	}
	return cucumber.Parse(f, lim)
}

// writeNew creates path with mode 0600, failing if anything already exists there.
func writeNew(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("report: write %s: %w", filepath.Base(path), err)
	}
	_, werr := f.Write(data)
	cerr := f.Close()
	if err := errors.Join(werr, cerr); err != nil {
		return fmt.Errorf("report: write %s: %w", filepath.Base(path), err)
	}
	return nil
}
