package pdf

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRound10EngineDiagnostics(t *testing.T) {
	for _, mode := range []string{"old", "failure"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("CUCUMBER_PDF_TEST_ENGINE", mode)
			_, err := Compress(t.Context(), fixturePath(t), filepath.Join(t.TempDir(), "out.pdf"), Options{Ghostscript: os.Args[0]})
			if err == nil || strings.Contains(err.Error(), "secret") {
				t.Fatalf("unsafe failure: %v", err)
			}
			want := "exit 1"
			if mode == "old" {
				want = "9.50 or newer"
			}
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("unhelpful error: %v", err)
			}
		})
	}
}

func TestRound10WindowsEngineDiscovery(t *testing.T) {
	for _, candidate := range []string{"gswin64c", "gswin32c"} {
		got, err := discoverEngine("", func(name string) (string, error) {
			if name == candidate {
				return name + ".exe", nil
			}
			return "", errors.New("missing")
		})
		if err != nil || got != candidate+".exe" {
			t.Fatal(got, err)
		}
	}
	var b cappedOutput
	payload := strings.Repeat("x", 8192)
	n, err := b.Write([]byte(payload))
	if n != 8192 || err != nil || len(b.data) != 4096 {
		t.Fatal("unbounded capture")
	}
}
