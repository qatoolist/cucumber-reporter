package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const malformedHookRun = `[{"uri":"features/a.feature","id":"f","keyword":"Feature","name":"F","line":1,"elements":[{"id":"f;s0","keyword":"Scenario","name":"s0","line":10,"type":"scenario","steps":[{"keyword":"Given ","name":"ok","line":11,"result":{"status":"failed","duration":1}}],"before":"x"}]}]`

func shardRun(uri string) string {
	return fmt.Sprintf(`[{"uri":%q,"id":"f","keyword":"Feature","name":"F","line":1,"elements":[{"id":"f;s0","keyword":"Scenario","name":"s0","line":10,"type":"scenario","steps":[{"keyword":"Given ","name":"ok","line":11,"result":{"status":"passed","duration":1}}]}]}]`, uri)
}

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// commandIn runs the real command with dir as its working directory, so that
// relative input arguments are exercised exactly as a user types them.
func commandIn(t *testing.T, dir string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), os.Args[0], args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CUCUMBER_REPORT_MAIN=1")
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return out.String(), errOut.String(), code
}

func TestRound17InputDiagnosticsPublished(t *testing.T) {
	root := writeTree(t, map[string]string{"r.json": malformedHookRun})
	out := t.TempDir()
	stdout, stderr, code := command(t, "-enhanced", "-o", out, filepath.Join(root, "r.json"))
	if code != 0 {
		t.Fatalf("%d %s", code, stderr)
	}
	const message = "Ignored malformed hook metadata"
	if strings.Count(stderr, message) != 1 {
		t.Fatalf("stderr should report the diagnostic once: %q", stderr)
	}
	index := reportPath(t, stdout)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(index), "summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	var summary struct {
		Diagnostics []struct{ Path, Message string }
	}
	if err = json.Unmarshal(b, &summary); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range summary.Diagnostics {
		found = found || (d.Path == "input[0].feature[0].elements[0].before" && d.Message == message)
	}
	if !found {
		t.Fatalf("summary.json lost the input diagnostic: %s", b)
	}
	html, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(html, []byte(message)) {
		t.Fatal("HTML report lost the input diagnostic")
	}
	stdout, _, code = command(t, "-validate", "-enhanced", filepath.Join(root, "r.json"))
	if code != 0 || !strings.Contains(stdout, "1 diagnostic") || strings.Contains(stdout, "2 diagnostics") {
		t.Fatalf("validate count: %d %q", code, stdout)
	}
}

func TestRound17SuffixNamesStayUnique(t *testing.T) {
	root := writeTree(t, map[string]string{
		"x/run.json":    shardRun("features/x.feature"),
		"ci/x/run.json": shardRun("features/cx.feature"),
		"run.json":      shardRun("features/r.feature"),
		"cfg.json":      `{"expectedShards":["x/run.json","ci/x/run.json","run.json"]}`,
	})
	out := t.TempDir()
	stdout, stderr, code := commandIn(t, root, "-enhanced", "-run-config", "cfg.json", "-o", out, "x/run.json", "ci/x/run.json", "run.json")
	if code != 0 {
		t.Fatalf("disjoint shards refused: %d %s", code, stderr)
	}
	b, err := os.ReadFile(filepath.Join(filepath.Dir(reportPath(t, stdout)), "summary.json"))
	if err != nil {
		t.Fatal(err)
	}
	var summary struct {
		Completeness  string
		MissingShards []string
	}
	if err = json.Unmarshal(b, &summary); err != nil {
		t.Fatal(err)
	}
	if len(summary.MissingShards) > 0 || summary.Completeness != "complete" {
		t.Fatalf("labels differ from the given relative paths: %s", b)
	}
}

func TestRound17ShortestInputName(t *testing.T) {
	for _, tc := range []struct {
		names []string
		want  []string
	}{
		{[]string{"x/run.json", "ci/x/run.json", "run.json"}, []string{"x/run.json", "ci/x/run.json", "run.json"}},
		{[]string{"x/run.json", "ci/x/run.json"}, []string{"x/run.json", "ci/x/run.json"}},
		{[]string{"a/r.json", "b/r.json"}, []string{"a/r.json", "b/r.json"}},
		{[]string{"/abs/x/run.json", "/other/abs/x/run.json"}, []string{"abs/x/run.json", "other/abs/x/run.json"}},
		{[]string{"./a/r.json", "c/a/r.json"}, []string{"a/r.json", "c/a/r.json"}},
	} {
		seen := map[string]bool{}
		for i, name := range tc.names {
			got := shortestInputName(name, tc.names)
			if got != tc.want[i] || filepath.IsAbs(got) || seen[got] {
				t.Errorf("%v[%d] = %q, want unique %q", tc.names, i, got, tc.want[i])
			}
			seen[got] = true
		}
	}
}

func TestRound17RepeatedArgumentWording(t *testing.T) {
	root := writeTree(t, map[string]string{"x.json": shardRun("features/x.feature")})
	_, stderr, code := commandIn(t, root, "-enhanced", "-o", t.TempDir(), "x.json", "x.json")
	if code != 1 || !strings.Contains(stderr, `input "x.json" given twice;`) || strings.Contains(stderr, "(as") {
		t.Fatalf("%d %s", code, stderr)
	}
	real, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	_, stderr, code = commandIn(t, root, "-enhanced", "-o", t.TempDir(), "x.json", filepath.Join(real, "x.json"))
	if code != 1 || !strings.Contains(stderr, "given twice (also as") {
		t.Fatalf("%d %s", code, stderr)
	}
}

func TestRound17MissingPlainResultsHint(t *testing.T) {
	empty := t.TempDir()
	_, stderr, code := command(t, "-o", t.TempDir(), empty)
	if code != 1 || strings.Contains(stderr, "PrepareRun") || !strings.Contains(stderr, "use -enhanced and list each shard as a separate input") {
		t.Fatalf("%d %s", code, stderr)
	}
}

func TestRound17StrictWordingConsistent(t *testing.T) {
	root := writeTree(t, map[string]string{
		"r.json":     malformedHookRun,
		"opt.json":   `{"version":2,"annotations":[{"scenario":"a-a8fde43a2452-L10","kind":"known","ref":"K","title":"t","step":1,"stepText":"other"}]}`,
		"plain.json": `{"version":2,"annotations":[{"scenario":"a-L10","kind":"known","ref":"K","title":"t","step":1,"stepText":"other"}]}`,
		"fail.json":  strings.Replace(malformedHookRun, `,"before":"x"`, "", 1),
	})
	for _, args := range [][]string{
		{"-validate", "-strict", "r.json"},
		{"-validate", "-strict", "-enhanced", "r.json"},
		{"-validate", "-strict", "-options", "plain.json", "fail.json"},
		{"-validate", "-strict", "-enhanced", "-options", "opt.json", "fail.json"},
	} {
		_, stderr, code := commandIn(t, root, args...)
		if code != 1 || !strings.HasSuffix(strings.TrimSpace(stderr), "cucumber-report: strict validation rejected run diagnostics") {
			t.Errorf("%v: %d %s", args, code, stderr)
		}
	}
}
