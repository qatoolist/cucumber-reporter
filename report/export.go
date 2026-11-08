package report

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	cucumber "github.com/qatoolist/cucumber-reporter/internal/cucumberinput"
)

type attemptData struct {
	Number           int        `json:"number"`
	Status           string     `json:"status"`
	Started          string     `json:"started,omitempty"`
	Finished         string     `json:"finished,omitempty"`
	Worker           string     `json:"worker,omitempty"`
	Steps            []stepData `json:"steps,omitempty"`
	UseScenarioSteps bool       `json:"useScenarioSteps,omitempty"`
}

// AttachmentManifest describes one evidence item in attachments.json without its
// content. Scenario is the navigation ID; Attempt is zero-based (-1 denotes the
// final scenario display), and Step is the one-based execution position, including
// hooks. State describes whether evidence
// is inline, in a sidecar or unavailable. File is a generated relative sidecar name
// when present; it is never a runner-supplied filesystem path.
type AttachmentManifest struct {
	Scenario string `json:"scenario"`
	Attempt  int    `json:"attempt"`
	Step     int    `json:"step"`
	Name     string `json:"name"`
	MIME     string `json:"mime"`
	Size     int64  `json:"size"`
	File     string `json:"file,omitempty"`
	State    string `json:"state"`
}

func extendPage(p *pageData, features []cucumber.Feature, opt renderOptions, sidecar sidecarFunc) error {
	attemptNumber := 0
	ids := scenarioIDs(features)
	if len(opt.execution) > 0 {
		p.Execution = map[string][]attemptData{}
	}
	for fi, f := range features {
		for si, sc := range f.Elements {
			for ti, st := range sc.Steps {
				if opt.BundleAttachments {
					for ai, e := range st.Embeddings {
						if p.Features[fi].Elements[si].Steps[ti].Att[ai].Data != nil || p.Features[fi].Elements[si].Steps[ti].Att[ai].Href != "" {
							continue
						}
						a, err := bundleAttachment(e, cucumber.Limits{Embedding: opt.Limits.WithDefaults().Embedding, InlineAttachment: -1}, sidecar)
						if err != nil {
							return err
						}
						p.Features[fi].Elements[si].Steps[ti].Att[ai] = a
					}
				}
			}
			attempts := opt.execution[ids[fi][si]].Attempts
			for attemptIndex, attempt := range attempts {
				attemptNumber++
				a := attemptData{Number: attempt.Number, Status: attempt.Status, Started: attempt.Started, Finished: attempt.Finished, Worker: attempt.Worker}
				a.UseScenarioSteps = attemptIndex == len(attempts)-1 && len(attempt.Steps) == 0
				for ti, st := range internalSteps(attempt.Steps, 0) {
					d, err := buildStep(st, -1, attemptNumber, ti, cucumber.Limits(opt.Limits), sidecar, &p.inlineRemaining)
					if err != nil {
						return err
					}
					if opt.BundleAttachments {
						for ai, e := range st.Embeddings {
							if d.Att[ai].Data != nil || d.Att[ai].Href != "" {
								continue
							}
							b, err := bundleAttachment(e, cucumber.Limits{Embedding: opt.Limits.WithDefaults().Embedding, InlineAttachment: -1}, sidecar)
							if err != nil {
								return err
							}
							d.Att[ai] = b
						}
					}
					a.Steps = append(a.Steps, d)
				}
				p.Execution[ids[fi][si]] = append(p.Execution[ids[fi][si]], a)
			}
		}
	}
	return nil
}

func bundleAttachment(e cucumber.Embedding, lim cucumber.Limits, sidecar sidecarFunc) (attData, error) {
	if int64(len(e.Data)) > lim.Embedding {
		return attData{}, cucumber.ErrTooLarge
	}
	a, err := buildAttachment(e, "unused", lim.InlineAttachment, nil)
	if err != nil {
		return a, err
	}
	if a.Data != nil || sidecar == nil {
		return a, nil
	}
	raw, err := e.Decode(lim.Embedding)
	if err != nil {
		return a, nil //nolint:nilerr // Preserve the explicit unavailable-evidence note from buildAttachment.
	} // malformed evidence stays explicitly unavailable
	sum := sha256.Sum256(raw)
	ext := ".bin"
	switch mediaType(e.MimeType) {
	case "image/png":
		ext = ".png"
	case "image/jpeg":
		ext = ".jpg"
	case "image/gif":
		ext = ".gif"
	case "image/webp":
		ext = ".webp"
	case "application/json":
		ext = ".json"
	default:
		if strings.HasPrefix(mediaType(e.MimeType), "text/") {
			ext = ".txt"
		}
	}
	name := "attachment-" + hex.EncodeToString(sum[:]) + ext
	if err = sidecar(name, raw); err != nil {
		return a, err
	}
	a.Href = name
	a.Note = ""
	return a, nil
}

// renderTo writes HTML directly to w without retaining a second complete output byte slice.
// The parsed model is still held in memory. A writer failure may leave partial output.
func renderTo(w io.Writer, features []cucumber.Feature, opt renderOptions) error {
	return renderWriter(w, features, opt, nil)
}

// writeRunDir atomically publishes an enriched run and exports in a fresh directory.
// ErrGateFailed is returned only after the complete artifact is published, with its directory.
func writeRunDir(run *preparedRun, parent string) (string, error) {
	if run == nil {
		return "", invalid("nil prepared run")
	}
	parent = filepath.Clean(parent)
	if err := requireOutputDir(parent); err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp(parent, ".html-report-")
	if err != nil {
		return "", err
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(tmp)
		}
	}()
	// Keep direct-library writer calls safe without mutating the caller's snapshot.
	snapshot := *run
	snapshot.Summary.Diagnostics = append([]Diagnostic(nil), run.Summary.Diagnostics...)
	run = &snapshot
	if !run.OmitHistory {
		archive, historyErr := compactHistory(run.History, 1000, HistoryByteLimit, scopeKey(run.Summary))
		if historyErr != nil {
			return "", historyErr
		}
		run.History = archive.entries
		run.OmitHistory = archive.omitted
		run.Summary.Diagnostics = append(run.Summary.Diagnostics, archive.diagnostics...)
	}
	run.Options.Analysis = &run.Summary
	opt := run.Options
	opt.execution = run.Metadata
	flusher := newFileFlusher(syncFileData)
	defer func() { _ = flusher.finish() }()
	sidecar := newSidecarWriter(tmp, flusher)
	doc, err := render(run.Features, opt, sidecar)
	if err != nil {
		return "", err
	}
	if err = sidecar("index.html", doc); err != nil {
		return "", err
	}
	outputs := map[string]any{"summary.json": run.Summary}
	if !run.OmitHistory {
		outputs["history.json"] = run.History
	}
	for name, value := range outputs {
		var b []byte
		b, err = json.Marshal(value)
		if err != nil {
			return "", err
		}
		if err = sidecar(name, append(b, '\n')); err != nil {
			return "", err
		}
	}
	if err = sidecar("scenarios.csv", scenarioCSV(run.Summary)); err != nil {
		return "", err
	}
	if err = sidecar("summary.md", []byte(summaryMarkdown(run.Summary))); err != nil {
		return "", err
	}
	if err = sidecar("rerun.txt", []byte(RerunLocations(run.Summary, nil))); err != nil {
		return "", err
	}
	manifest, err := runManifest(run, opt, func(name string, data []byte) error {
		written, readErr := os.ReadFile(filepath.Join(tmp, name))
		if readErr != nil {
			return readErr
		}
		if !bytes.Equal(written, data) {
			return invalid("manifest sidecar differs from rendered evidence")
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if manifest == nil {
		manifest = []AttachmentManifest{}
	}
	b, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	if err = sidecar("attachments.json", append(b, '\n')); err != nil {
		return "", err
	}
	if run.Bundle {
		if err = writeBundle(tmp); err != nil {
			return "", err
		}
	}
	final := filepath.Join(parent, "html-"+strings.TrimPrefix(filepath.Base(tmp), ".html-report-"))
	if _, err = os.Lstat(final); !os.IsNotExist(err) {
		return "", invalid("output already exists")
	}
	if err = flusher.finish(); err != nil {
		return "", err
	}
	if err = syncDir(tmp); err != nil {
		return "", err
	}
	if err = os.Rename(tmp, final); err != nil {
		return "", err
	}
	if err = syncDir(parent); err != nil {
		_ = os.RemoveAll(final)
		_ = syncDir(parent)
		return "", err
	}
	published = true
	if len(run.Summary.GateFailures) > 0 {
		return final, ErrGateFailed
	}
	return final, nil
}

func runManifest(run *preparedRun, opt renderOptions, sidecar sidecarFunc) ([]AttachmentManifest, error) {
	data, err := buildPage(run.Features, opt, sidecar)
	if err != nil {
		return nil, err
	}
	if err = extendPage(data, run.Features, opt, sidecar); err != nil {
		return nil, err
	}
	var entries []AttachmentManifest
	add := func(id string, attempt int, steps []stepData) {
		for ti, st := range steps {
			for _, a := range st.Att {
				state := "omitted"
				if a.Data != nil {
					state = "inline"
				} else if a.Href != "" {
					state = "sidecar"
				} else if strings.Contains(a.Note, "base64") {
					state = "invalid"
				}
				entries = append(entries, AttachmentManifest{id, attempt, ti + 1, a.Name, a.Mime, a.Size, a.Href, state})
			}
		}
	}
	for _, f := range data.Features {
		for _, s := range f.Elements {
			add(s.ID, -1, s.Steps)
		}
	}
	executionIDs := make([]string, 0, len(data.Execution))
	for id := range data.Execution {
		executionIDs = append(executionIDs, id)
	}
	sort.Strings(executionIDs)
	for _, id := range executionIDs {
		attempts := data.Execution[id]
		for _, a := range attempts {
			add(id, a.Number, a.Steps)
		}
	}
	return entries, nil
}

func csvSafe(s string) string {
	s = clean(s)
	trimmed := strings.TrimLeftFunc(s, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) || r == '\ufeff' })
	if strings.ContainsAny(trimmed[:min(1, len(trimmed))], "=+-@") || strings.HasPrefix(s, "\t") || strings.HasPrefix(s, "\r") {
		return "'" + s
	}
	return s
}

func scenarioCSV(s RunSummary) []byte {
	var b bytes.Buffer
	w := csv.NewWriter(&b)
	_ = w.Write([]string{"identity", "scenario", "uri", "line", "status", "duration_ns", "change", "attempts"})
	for _, x := range s.Scenarios {
		_ = w.Write([]string{csvSafe(x.Identity), csvSafe(x.Name), csvSafe(x.URI), scenarioLine(x), x.Status, strconv.FormatInt(x.Duration, 10), x.Change, strconv.Itoa(max(1, len(x.Attempts)))})
	}
	w.Flush()
	return b.Bytes()
}

func markdownText(s string) string {
	r := strings.NewReplacer("\\", "\\\\", "`", "\\`", "*", "\\*", "_", "\\_", "[", "\\[", "<", "&lt;", ">", "&gt;", "\n", " ", "\r", " ")
	return r.Replace(clean(s))
}

func summaryMarkdown(s RunSummary) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n%s\n\nCompleteness: %s. Verdict source: %s.\n\n", markdownText(s.Name), markdownText(s.Computed.Label), s.Completeness, s.VerdictSource)
	for _, k := range statusOrder {
		if n := s.Counts[k]; n > 0 {
			fmt.Fprintf(&b, "- %s: %d\n", k, n)
		}
	}
	if s.Comparison != nil {
		fmt.Fprintf(&b, "\nCompared with %s: %d new failures, %d fixed, %d missing.\n", markdownText(s.Comparison.Baseline), len(s.Comparison.NewFailures), len(s.Comparison.Fixed), len(s.Comparison.Missing))
	}
	for _, x := range s.Scenarios {
		if failedOutcome(x.Status) {
			location := markdownText(x.URI)
			if x.RunHook {
				location = "Run hooks"
			}
			if x.Line > 0 && !x.RunHook {
				location += ":" + strconv.Itoa(x.Line)
			}
			fmt.Fprintf(&b, "\n- %s — %s (%s)\n", markdownText(x.Name), x.Status, location)
		}
	}
	for _, g := range s.GateFailures {
		fmt.Fprintf(&b, "\nGate: %s\n", g)
	}
	return b.String()
}

// RerunLocations returns unique feature:line selectors. Nil ids selects unexpected problems.
// Locations containing newlines or no positive line are omitted; no executable shell is generated.
func RerunLocations(s RunSummary, ids []string) string {
	selected := map[string]bool{}
	for _, id := range ids {
		selected[id] = true
	}
	seen := map[string]bool{}
	var b strings.Builder
	for _, x := range s.Scenarios {
		if (ids == nil && !failedOutcome(x.Status) && !scenarioGateFailed(x, s.GateFailures)) || (ids != nil && !selected[x.ID]) || x.RunHook || x.RerunUnavailable || x.Line <= 0 || strings.ContainsAny(x.URI, "\r\n") {
			continue
		}
		loc := fmt.Sprintf("%s:%d", clean(x.URI), x.Line)
		if !seen[loc] {
			b.WriteString(loc + "\n")
			seen[loc] = true
		}
	}
	return b.String()
}

func writeBundle(dir string) error {
	f, err := os.OpenFile(filepath.Join(dir, "report.zip"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	z := zip.NewWriter(f)
	entries, err := os.ReadDir(dir)
	if err != nil {
		_ = z.Close()
		_ = f.Close()
		return err
	}
	for _, e := range entries {
		if e.Name() == "report.zip" || e.IsDir() {
			continue
		}
		source, openErr := os.Open(filepath.Join(dir, e.Name()))
		if openErr != nil {
			_ = z.Close()
			_ = f.Close()
			return openErr
		}
		entry, entryErr := z.Create(e.Name())
		if entryErr == nil {
			_, entryErr = io.Copy(entry, source)
		}
		closeErr := source.Close()
		if entryErr = errors.Join(entryErr, closeErr); entryErr != nil {
			_ = z.Close()
			_ = f.Close()
			return entryErr
		}
	}
	if err = z.Close(); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

var contentAttachmentName = regexp.MustCompile(`^attachment-[a-f0-9]{64}\.(bin|png|jpg|gif|webp|json|txt)$`)

func newSidecarWriter(dir string, flusher ...*fileFlusher) sidecarFunc {
	write := writeNew
	if len(flusher) > 0 {
		write = flusher[0].write
	}
	written := map[string]bool{}
	return func(name string, data []byte) error {
		if written[name] {
			if contentAttachmentName.MatchString(name) {
				return nil
			}
			return fmt.Errorf("report: duplicate sidecar: %w", os.ErrExist)
		}
		if err := write(filepath.Join(dir, name), data); err != nil {
			return err
		}
		written[name] = true
		return nil
	}
}

func scenarioLine(s ScenarioSummary) string {
	if s.Line <= 0 {
		return ""
	}
	return strconv.Itoa(s.Line)
}

func scenarioGateFailed(s ScenarioSummary, failures []string) bool {
	for _, f := range failures {
		if f == "pending scenarios" && s.Status == "pending" {
			return true
		}
		if f == "flaky scenarios" && s.Retried && (s.Status == "passed" || s.Status == "known") {
			for _, a := range s.Attempts {
				if a.Status != "passed" {
					return true
				}
			}
		}
	}
	return false
}
