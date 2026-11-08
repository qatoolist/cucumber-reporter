// Package report renders parsed Cucumber results as one self-contained, offline HTML document.
//
// The report data travels in one <script type="application/json"> block written by encoding/json with
// HTML-safe escaping; the embedded page script builds the interface from it with createElement and
// textContent only. A no-JS fallback is rendered by html/template from the same view model. The page
// carries a Content-Security-Policy that admits only the embedded stylesheet and script by their
// SHA-256 hashes, so no markup, style or script can come from the data. Viewer preferences are stored
// in the browser's localStorage.
package report

//go:generate go run ../internal/cmd/minify assets/app.js assets/app.min.js
//go:generate go run ../internal/cmd/minify assets/app.css assets/app.min.css

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"

	modelmeta "github.com/qatoolist/cucumber-reporter/internal/cucumbermodel"

	cucumber "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

var (
	// Only the minified assets are embedded; TestMinifiedAssetsAreCurrent keeps them equal to the
	// minified readable sources.
	//go:embed assets/app.min.css
	styleSource string
	//go:embed assets/app.min.js
	scriptSource string
	//go:embed templates/page.gohtml
	templateFS embed.FS

	page = template.Must(template.New("page.gohtml").Funcs(template.FuncMap{
		"label": func(s string) string { return statusLabels[s] },
		"toneClass": func(t string) string {
			return map[string]string{ToneFail: "s-failed", ToneWarn: "s-known", ToneOK: "s-passed"}[t]
		},
		"open":             func(s string) bool { return s == "failed" || s == "ambiguous" || s == "undefined" },
		"join":             func(tags []string) string { return strings.Join(tags, " ") },
		"trim":             strings.TrimSpace,
		"fallbackFeatures": fallbackFeatures,
		"dur": func(v any) string {
			switch ns := v.(type) {
			case int64:
				return formatNS(ns)
			case *int64:
				return formatNS(*ns)
			}
			return ""
		},
	}).ParseFS(templateFS, "templates/page.gohtml"))

	// head and script are fixed at package initialisation from the embedded assets only. They are the
	// only template.HTML values in the package; report data never becomes template.HTML.
	head, script = assetElements()
)

// pageView is the template's root: the constant asset elements, the view model for the fallback, and
// the same view model as Data, which html/template marshals with encoding/json into the data block.
type pageView struct {
	Head, Script template.HTML
	*pageData
	Data *pageData
}

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
	// Built only from the embedded assets and their hashes; no report data reaches these values.
	return template.HTML(h), template.HTML("<script>" + scriptSource + "</script>") //nolint:gosec // G203: constant embedded assets, never data
}

// sidecarFunc stores an oversize text attachment beside the page under a renderer-chosen name.
type sidecarFunc func(name string, data []byte) error

// renderPublicModel returns one HTML document for features. It is deterministic: the same input and options
// produce the same bytes. Text attachments larger than renderOptions.Limits.InlineAttachment are left out
// with a notice; renderDir writes them beside the page instead.
func renderPublicModel(features []cucumber.Feature, opt renderOptions) ([]byte, error) {
	return render(features, opt, nil)
}

func render(features []cucumber.Feature, opt renderOptions, sidecar sidecarFunc) ([]byte, error) {
	var buf bytes.Buffer
	if err := renderWriter(&buf, features, opt, sidecar); err != nil {
		return nil, err
	}
	if buf.Len() == 0 {
		return nil, ErrEmptyOutput
	}
	return buf.Bytes(), nil
}

func renderWriter(w io.Writer, features []cucumber.Feature, opt renderOptions, sidecar sidecarFunc) error {
	opt, err := normalize(opt)
	if err != nil {
		return err
	}
	if opt.renderContext == nil {
		opt.renderContext = context.Background()
	}
	if err = opt.renderContext.Err(); err != nil {
		return err
	}
	w = contextWriter{ctx: opt.renderContext, w: w}
	if opt.Analysis == nil {
		features, err = modelmeta.PlainFeatures(features)
		if err != nil {
			return err
		}
	}
	if err = validateRenderFeatures(features); err != nil {
		return err
	}
	if opt.Analysis == nil {
		ids := scenarioIDs(features)
		for fi, f := range features {
			for si, sc := range f.Elements {
				previous := modelmeta.ScenarioMetadata(&sc).PreviousAttempts
				if len(previous) == 0 {
					continue
				}
				if opt.execution == nil {
					opt.execution = map[string]ScenarioMeta{}
				}
				m := ScenarioMeta{}
				for n, attempt := range append(append([]cucumber.Scenario(nil), previous...), sc) {
					attemptElements := []cucumber.Scenario{attempt}
					if !modelmeta.ScenarioMetadata(&attempt).PlainFolded && len(modelmeta.ScenarioMetadata(&attempt).AttemptBackground) > 0 {
						attemptElements = append([]cucumber.Scenario{{Type: "background", Steps: modelmeta.ScenarioMetadata(&attempt).AttemptBackground}}, attemptElements...)
					}
					folded, e := modelmeta.PlainFeatures([]cucumber.Feature{{Elements: attemptElements}})
					if e != nil {
						return e
					}
					st := folded[0].Elements[0]
					m.Attempts = append(m.Attempts, Attempt{Number: n, Status: string(modelmeta.ExecutionStatus(st)), Steps: publicSteps(st.Steps, 0)})
				}
				opt.execution[ids[fi][si]] = m
			}
		}
	}
	for i, f := range features {
		if modelmeta.FeatureMetadata(&f).OrphanBackground {
			opt.Sections = append(opt.Sections, Section{Title: "Input diagnostics", Rows: []Fact{{Label: fmt.Sprintf("feature[%d]", i), Value: "Orphan Background: non-passing evidence is retained; passing-only results are not counted"}}})
		}
	}
	buildSidecar := sidecar
	data, err := buildPage(features, opt, buildSidecar)
	if err != nil {
		return err
	}
	if err = extendPage(data, features, opt, sidecar); err != nil {
		return err
	}
	if err = checkDisplayedEvidence(features, data); err != nil {
		return err
	}
	if err = page.Execute(w, pageView{Head: head, Script: script, pageData: data, Data: data}); err != nil {
		return fmt.Errorf("report: execute template: %w", err)
	}
	return nil
}

// renderDir reads <runDir>/cucumber.json and writes a new <runDir>/html-XXXXXX/index.html, returning
// the new directory. See renderDirTo.
func renderDir(runDir string, opt renderOptions) (string, error) {
	return renderDirTo(runDir, runDir, opt)
}

// renderDirTo reads <runDir>/cucumber.json and writes a new <parentDir>/html-XXXXXX/index.html,
// returning the new directory. Neither runDir nor cucumber.json may be a symbolic link, and
// cucumber.json must be a regular file. See renderFeaturesDirTo for the output rules.
func renderDirTo(runDir, parentDir string, opt renderOptions) (string, error) {
	opt, err := normalize(opt)
	if err != nil {
		return "", err
	}
	runDir = filepath.Clean(runDir)
	if info, e := os.Stat(runDir); e == nil && info.Mode().IsRegular() {
		return "", &inputHint{ErrNotRegular, "plain mode takes the directory containing cucumber.json; pass a file with -enhanced"}
	}
	if err = requireDir(runDir); err != nil {
		return "", err
	}
	if err = requireOutputDir(filepath.Clean(parentDir)); err != nil {
		return "", err
	}
	features, err := readResults(filepath.Join(runDir, "cucumber.json"), cucumber.Limits(opt.Limits))
	if err != nil {
		return "", err
	}
	return renderFeaturesDirTo(features, parentDir, opt)
}

// renderFeaturesDirTo writes already-parsed features as a new <parentDir>/html-XXXXXX/index.html and
// returns the new directory. parentDir may not be a symbolic link (however it is spelled; the path is
// cleaned first, so "link/" and "link/." are refused too). The report is written into a temporary
// directory inside parentDir (mode 0700, files 0600) and then renamed; earlier reports are never
// replaced, and a failed render leaves nothing behind. Oversize text attachments are written beside
// index.html as .txt or .json files and linked. Symbolic links in parent components of the path are
// followed.
func renderFeaturesDirTo(features []cucumber.Feature, parentDir string, opt renderOptions) (string, error) {
	opt, err := normalize(opt)
	if err != nil {
		return "", err
	}
	parentDir = filepath.Clean(parentDir)
	if err = requireOutputDir(parentDir); err != nil {
		return "", err
	}
	if opt.Analysis == nil {
		features, err = modelmeta.PlainFeatures(features)
		if err != nil {
			return "", err
		}
	}
	if err = validateRenderFeatures(features); err != nil {
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
	if err = os.Chmod(tmp, 0o700); err != nil { //nolint:gosec // G302: a directory needs its execute bit; 0700 is owner-only
		return "", fmt.Errorf("report: create output directory: %w", err)
	}
	file, err := os.OpenFile(filepath.Join(tmp, "index.html"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	flusher := newFileFlusher(syncFileData)
	defer func() { _ = flusher.finish() }()
	renderErr := renderWriter(file, features, opt, newSidecarWriter(tmp, flusher))
	if renderErr != nil {
		return "", errors.Join(renderErr, file.Close())
	}
	flusher.queue(file)
	if err = flusher.finish(); err != nil {
		return "", err
	}
	if err = syncDir(tmp); err != nil {
		return "", err
	}

	// The random suffix comes from MkdirTemp, so the final name is new unless another process created
	// it in the meantime; rename(2) cannot replace a non-empty directory either way.
	final := filepath.Join(parentDir, "html-"+strings.TrimPrefix(filepath.Base(tmp), ".html-report-"))
	if _, statErr := os.Lstat(final); !errors.Is(statErr, os.ErrNotExist) {
		return "", fmt.Errorf("report: output directory %s already exists", final)
	}
	if err = os.Rename(tmp, final); err != nil {
		return "", fmt.Errorf("report: publish output directory: %w", err)
	}
	if err = syncDir(parentDir); err != nil {
		_ = os.RemoveAll(final)
		_ = syncDir(parentDir)
		return "", err
	}
	published = true
	return final, nil
}

// openResults opens cucumber.json; tests replace it to simulate a file swapped after inspection.
var openResults = os.Open

func requireOutputDir(dir string) error {
	err := requireDir(dir)
	if errors.Is(err, ErrNoInput) {
		return fmt.Errorf("%w: %s; create it before rendering", ErrNoOutputDir, dir)
	}
	return err
}

func requireDir(dir string) error {
	info, err := os.Lstat(dir)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return &inputHint{ErrNoInput, fmt.Sprintf("input directory not found: %s", dir)}
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
		return nil, &inputHint{ErrNoInput, fmt.Sprintf("report: %q not found", path)}
	case err != nil:
		return nil, fmt.Errorf("report: inspect cucumber.json: %w", err)
	case !info.Mode().IsRegular():
		return nil, fmt.Errorf("%w: %s", ErrNotRegular, path)
	}
	f, err := openResults(path)
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

// validateModel checks the same model and annotation rules as plain rendering without
// writing HTML or sidecars. Input validity is separate from the run's verdict.
func validateModel(features []cucumber.Feature, opt renderOptions) error {
	var err error
	opt, err = normalize(opt)
	if err != nil {
		return err
	}
	features, err = modelmeta.PlainFeatures(features)
	if err != nil {
		return err
	}
	if err = validateRenderFeatures(features); err != nil {
		return err
	}
	_, err = buildPage(features, opt, nil)
	return err
}

type contextWriter struct {
	ctx context.Context
	w   io.Writer
}

func (w contextWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	return w.w.Write(p)
}

// The static fallback bounds passed detail while retaining every problem. The
// interactive data and print/export scopes always retain the complete run.
func fallbackFeatures(features []featureData) []featureData {
	n := 0
	for _, f := range features {
		n += len(f.Elements)
	}
	if n <= 1000 {
		return features
	}
	out := make([]featureData, 0, len(features))
	kept := 0
	for _, f := range features {
		scenarios := f.Elements
		f.Elements = nil
		for _, sc := range scenarios {
			if sc.Status != "passed" || kept < 1000 {
				f.Elements = append(f.Elements, sc)
				kept++
			}
		}
		if len(f.Elements) > 0 {
			out = append(out, f)
		}
	}
	return out
}

func validateRenderFeatures(features []cucumber.Feature) error {
	for i, f := range features {
		if len(f.Elements) == 0 && modelmeta.FeatureMetadata(&f).OrphanBackground {
			continue
		}
		if err := cucumber.Validate([]cucumber.Feature{f}); err != nil {
			return fmt.Errorf("report: feature[%d]: %w", i, err)
		}
	}
	return nil
}

type inputHint struct {
	cause   error
	message string
}

func (e *inputHint) Error() string { return e.message }
func (e *inputHint) Unwrap() error { return e.cause }
