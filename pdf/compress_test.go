package pdf

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if mode := os.Getenv("CUCUMBER_PDF_TEST_ENGINE"); mode != "" {
		args := strings.Join(os.Args[1:], " ")
		if args == "--version" {
			if mode == "old" {
				fmt.Println("9.49")
			} else {
				fmt.Println("10.08.0")
			}
			os.Exit(0)
		}
		if strings.Contains(args, "-dNODISPLAY") {
			if mode == "page-loss" && strings.Contains(args, "PDFPath=compressed.pdf") {
				fmt.Println("2")
			} else {
				fmt.Println("3")
			}
			os.Exit(0)
		}
		if mode == "failure" {
			fmt.Fprintln(os.Stderr, "secret report content must not escape")
			os.Exit(1)
		}
		data, err := os.ReadFile("input.pdf")
		if err != nil {
			os.Exit(2)
		}
		if mode == "larger" {
			data = append(data, bytes.Repeat([]byte(" "), 1024)...)
		}
		if err = os.WriteFile("compressed.pdf", data, 0o600); err != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func fixturePath(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "testdata", "synthetic", "pdf", "uncompressed.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCompressPreservesPagesTextAndOriginal(t *testing.T) {
	engine := os.Getenv("GS")
	if engine == "" {
		engine = "gs"
	}
	resolved, err := exec.LookPath(engine)
	if err != nil {
		if os.Getenv("CI") != "" || os.Getenv("REQUIRE_GS") != "" {
			t.Fatal("Ghostscript is required for PDF integration checks")
		}
		t.Skip("Ghostscript not installed; CI installs it for PDF integration checks")
	}
	input := fixturePath(t)
	before, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, quality := range []Quality{High, Balanced, Small} {
		t.Run(string(quality), func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "shared.pdf")
			result, err := Compress(t.Context(), input, output, Options{Quality: quality, Ghostscript: resolved})
			if err != nil {
				t.Fatal(err)
			}
			if result.Pages != 3 || !result.Reduced || result.OutputBytes >= result.InputBytes/2 {
				t.Fatalf("unexpected result: %+v", result)
			}
			after, err := os.ReadFile(input)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("source modified", err)
			}
			info, err := os.Stat(output)
			if err != nil {
				t.Fatal(err)
			}
			if info.Size() != result.OutputBytes {
				t.Fatal("size reporting incorrect")
			}
			if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
				t.Fatalf("output permissions %o", info.Mode().Perm())
			}
			text, err := exec.CommandContext(t.Context(), resolved, "-q", "-dSAFER", "-dBATCH", "-dNOPAUSE", "-sDEVICE=txtwrite", "-sOutputFile=-", "-f", output).Output()
			if err != nil {
				t.Fatal(err)
			}
			for page := 1; page <= 3; page++ {
				for _, want := range []string{fmt.Sprintf("PAGE_SENTINEL_%d", page), fmt.Sprintf("Scenario %d-32: PASSED", page)} {
					if !bytes.Contains(text, []byte(want)) {
						t.Errorf("missing %s", want)
					}
				}
			}
			entries, err := os.ReadDir(filepath.Dir(output))
			if err != nil || len(entries) != 1 {
				t.Fatal("temporary files retained", err)
			}
		})
	}
}

func TestCompressNeverEnlargesOrOverwrites(t *testing.T) {
	t.Setenv("CUCUMBER_PDF_TEST_ENGINE", "larger")
	input := fixturePath(t)
	output := filepath.Join(t.TempDir(), "shared.pdf")
	result, err := Compress(t.Context(), input, output, Options{Ghostscript: os.Args[0]})
	if err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(input)
	saved, _ := os.ReadFile(output)
	if result.Reduced || result.InputBytes != result.OutputBytes || !bytes.Equal(original, saved) {
		t.Fatalf("enlarged output: %+v", result)
	}
	if _, err = Compress(t.Context(), input, output, Options{Ghostscript: os.Args[0]}); err == nil {
		t.Fatal("overwrote output")
	}
	savedAgain, _ := os.ReadFile(output)
	if !bytes.Equal(saved, savedAgain) {
		t.Fatal("existing output changed")
	}
}

func TestCompressFailureLeavesNoOutput(t *testing.T) {
	for _, mode := range []string{"page-loss", "failure"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("CUCUMBER_PDF_TEST_ENGINE", mode)
			dir := t.TempDir()
			output := filepath.Join(dir, "shared.pdf")
			_, err := Compress(t.Context(), fixturePath(t), output, Options{Ghostscript: os.Args[0]})
			if err == nil || strings.Contains(err.Error(), "secret") {
				t.Fatalf("invalid failure: %v", err)
			}
			entries, readErr := os.ReadDir(dir)
			if readErr != nil || len(entries) != 0 {
				t.Fatal("failed compression published or retained files", readErr)
			}
		})
	}
}

func TestCompressValidation(t *testing.T) {
	input := fixturePath(t)
	dir := t.TempDir()
	bad := filepath.Join(dir, "not-pdf")
	if err := os.WriteFile(bad, []byte("private text"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, input, output string
		opt                 Options
	}{
		{"same path", input, input, Options{}},
		{"invalid quality", input, filepath.Join(dir, "out.pdf"), Options{Quality: "fast"}},
		{"bad extension", input, filepath.Join(dir, "out.txt"), Options{}},
		{"missing engine", input, filepath.Join(dir, "out.pdf"), Options{Ghostscript: filepath.Join(dir, "missing")}},
		{"not PDF", bad, filepath.Join(dir, "out.pdf"), Options{Ghostscript: os.Args[0]}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Compress(t.Context(), tc.input, tc.output, tc.opt); err == nil {
				t.Fatal("invalid request accepted")
			}
		})
	}
	if runtime.GOOS != "windows" {
		link := filepath.Join(dir, "link.pdf")
		if err := os.Symlink(input, link); err != nil {
			t.Fatal(err)
		}
		if _, err := Compress(t.Context(), link, filepath.Join(dir, "out.pdf"), Options{Ghostscript: os.Args[0]}); err == nil {
			t.Fatal("followed input symlink")
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Compress(ctx, input, filepath.Join(dir, "out.pdf"), Options{Ghostscript: os.Args[0]}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestRejectEncryptedAndRestrictedPDF(t *testing.T) {
	output := filepath.Join(t.TempDir(), "shared.pdf")
	input := filepath.Join("..", "testdata", "synthetic", "pdf", "owner-restricted.pdf")
	_, err := Compress(t.Context(), input, output, Options{Ghostscript: os.Args[0]})
	if err == nil || !strings.Contains(err.Error(), "protection") {
		t.Fatalf("restricted PDF accepted: %v", err)
	}
	if _, err = os.Stat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("restricted PDF published")
	}
	for _, s := range []string{"<</Encrypt 7 0 R>>", "<</Encr#79pt 7 0 R>>", "<</#45ncrypt 7 0 R>>"} {
		if err = rejectEncryption(strings.NewReader(s)); err == nil {
			t.Fatalf("missed encryption name: %s", s)
		}
	}
	if err = rejectEncryption(strings.NewReader("<</EncryptedText (sample)>>")); err != nil {
		t.Fatal(err)
	}
}

func TestPublishWithoutHardLinks(t *testing.T) {
	noLink := func(string, string) error { return errors.New("hard links unsupported") }
	source := fixturePath(t)
	output := filepath.Join(t.TempDir(), "out.pdf")
	if err := publishPDF(t.Context(), source, output, noLink); err != nil {
		t.Fatal(err)
	}
	expected, _ := os.ReadFile(source)
	got, _ := os.ReadFile(output)
	if !bytes.Equal(expected, got) {
		t.Fatal("fallback copy changed bytes")
	}
	if err := publishPDF(t.Context(), source, output, noLink); err == nil {
		t.Fatal("fallback overwrote existing file")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	canceled := filepath.Join(t.TempDir(), "out.pdf")
	if err := publishPDF(ctx, source, canceled, noLink); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := os.Stat(canceled); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("canceled fallback left a partial PDF")
	}
}
