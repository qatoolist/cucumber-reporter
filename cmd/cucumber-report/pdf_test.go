package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCompressPDFCommand(t *testing.T) {
	engine := os.Getenv("GS")
	if engine == "" {
		engine = "gs"
	}
	if _, err := exec.LookPath(engine); err != nil {
		if os.Getenv("CI") != "" || os.Getenv("REQUIRE_GS") != "" {
			t.Fatal("Ghostscript is required for PDF CLI integration checks")
		}
		t.Skip("Ghostscript not installed")
	}
	input := filepath.Join("..", "..", "testdata", "synthetic", "pdf", "uncompressed.pdf")
	output := filepath.Join(t.TempDir(), "shared report.pdf")
	stdout, stderr, code := command(t, "compress-pdf", "-quality", "balanced", "-o", output, input)
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	for _, want := range []string{"PDF: " + output, "Pages: 3 (all retained)", "Quality: balanced", "smaller)"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("missing %q in %q", want, stdout)
		}
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatal(err)
	}
	_, _, code = command(t, "compress-pdf", "-o", output, input)
	if code != 1 {
		t.Fatal("existing output was not refused")
	}
}

func TestCompressPDFUsage(t *testing.T) {
	for _, args := range [][]string{{"compress-pdf"}, {"compress-pdf", "-quality", "unknown", "report.pdf"}, {"compress-pdf", "one.pdf", "two.pdf"}} {
		stdout, stderr, code := command(t, args...)
		if code != 2 || stdout != "" || !strings.Contains(stderr, "usage: cucumber-report compress-pdf") {
			t.Fatalf("invalid usage: %d %s %s", code, stdout, stderr)
		}
	}
}

func TestCompressPDFInterruptCleansTemporaryCopy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX interrupt fixture")
	}
	engineDir := t.TempDir()
	engine := filepath.Join(engineDir, "gs-test")
	marker := filepath.Join(engineDir, "started")
	script := "#!/bin/sh\ncase \"$*\" in *-dNODISPLAY*) printf '3\\n';; *) : > \"$PDF_TEST_STARTED\"; exec sleep 30;; esac\n"
	if err := os.WriteFile(engine, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	outputDir := t.TempDir()
	output := filepath.Join(outputDir, "shared.pdf")
	input := filepath.Join("..", "..", "testdata", "synthetic", "pdf", "uncompressed.pdf")
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "compress-pdf", "-o", output, input)
	cmd.Env = append(os.Environ(), "CUCUMBER_REPORT_MAIN=1", "GS="+engine, "PDF_TEST_STARTED="+marker)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill() }()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if _, err := os.Stat(marker); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("compressor did not start")
		case <-ticker.C:
		}
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	var exitErr *exec.ExitError
	if err := cmd.Wait(); !errors.As(err, &exitErr) || exitErr.ExitCode() != 130 {
		t.Fatalf("unexpected interrupt exit: %v", err)
	}
	entries, err := os.ReadDir(outputDir)
	if err != nil || len(entries) != 0 {
		t.Fatal("interrupt left temporary PDF data", err)
	}
}
