// Package pdf compresses saved report PDFs locally using an optional Ghostscript executable.
// It preserves the page count and never overwrites an existing file.
package pdf

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Quality controls image downsampling; text and vector graphics remain vector content.
type Quality string

const (
	// High compresses resources without downsampling images.
	High Quality = "high"
	// Balanced downsamples color and grayscale images to 150 dpi for sharing.
	Balanced Quality = "balanced"
	// Small downsamples color and grayscale images to 96 dpi for smaller screen copies.
	Small Quality = "small"
)

// Options selects the quality and, optionally, an explicit Ghostscript executable.
// Ghostscript defaults to GS or the gs executable on PATH. No tool is downloaded.
type Options struct {
	Quality     Quality
	Ghostscript string
}

// Result reports the actual saved size. Reduced is false when the original was
// already smaller than the recompressed candidate; the output is then an exact copy.
type Result struct {
	InputBytes  int64
	OutputBytes int64
	Pages       int
	Reduced     bool
}

const maxInputBytes int64 = 1 << 30

// Compress writes a fresh PDF at output. It retains every page, verifies the page
// count after conversion and publishes the smaller of the original and candidate.
// Input must be a regular PDF file of at most 1 GiB. Output must not already exist.
// A caller deadline bounds Ghostscript; the CLI supplies a five-minute deadline.
func Compress(ctx context.Context, input, output string, opt Options) (Result, error) {
	var result Result
	if opt.Quality == "" {
		opt.Quality = Balanced
	}
	if opt.Quality != High && opt.Quality != Balanced && opt.Quality != Small {
		return result, errors.New("pdf: quality must be high, balanced or small")
	}
	input, err := filepath.Abs(input)
	if err != nil {
		return result, errors.New("pdf: invalid input path")
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return result, errors.New("pdf: invalid output path")
	}
	if input == output {
		return result, errors.New("pdf: output must differ from input")
	}
	if _, err = os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
		return result, errors.New("pdf: output already exists or cannot be inspected")
	}
	if strings.ToLower(filepath.Ext(output)) != ".pdf" {
		return result, errors.New("pdf: output needs a .pdf extension")
	}
	engine := opt.Ghostscript
	if engine == "" {
		engine = os.Getenv("GS")
	}
	engine, err = discoverEngine(engine, exec.LookPath)
	if err != nil {
		return result, err
	}

	// Resolve before changing the subprocess working directory.
	engine, err = filepath.Abs(engine)
	if err != nil {
		return result, errors.New("pdf: invalid Ghostscript path")
	}
	info, err := os.Lstat(input)
	if err != nil || !info.Mode().IsRegular() {
		return result, errors.New("pdf: input must be a regular file, not a symbolic link")
	}
	if info.Size() > maxInputBytes {
		return result, errors.New("pdf: input exceeds 1 GiB")
	}
	source, err := os.Open(input)
	if err != nil {
		return result, errors.New("pdf: cannot read input")
	}
	defer func() { _ = source.Close() }()
	opened, err := source.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return result, errors.New("pdf: input changed while opening")
	}
	tmp, err := os.MkdirTemp(filepath.Dir(output), ".pdf-compress-")
	if err != nil {
		return result, errors.New("pdf: cannot create output workspace")
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	snapshot := filepath.Join(tmp, "input.pdf")
	result.InputBytes, err = copySnapshot(snapshot, contextReader{ctx, source})
	if err != nil {
		return result, err
	}
	if err = checkEngineVersion(ctx, engine); err != nil {
		return result, err
	}
	// Only fixed local filenames enter the interpreter; caller paths are never code or flags.
	result.Pages, err = pageCount(ctx, engine, tmp, "input.pdf")
	if err != nil {
		return result, fmt.Errorf("pdf: cannot inspect input PDF (it may be invalid): %w", err)
	}
	args := compressionArgs(opt.Quality)
	cmd := exec.CommandContext(ctx, engine, args...) //nolint:gosec // Explicit operator-selected executable; no shell and fixed arguments.
	cmd.Dir = tmp
	if _, err = runEngine(ctx, cmd); err != nil {
		return result, err
	}

	candidate := filepath.Join(tmp, "compressed.pdf")
	info, err = os.Lstat(candidate)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return result, errors.New("pdf: compressor produced no PDF")
	}
	pages, err := pageCount(ctx, engine, tmp, "compressed.pdf")
	if err != nil {
		return result, err
	}
	if pages != result.Pages {
		return result, errors.New("pdf: compressed page count changed; no output published")
	}
	result.OutputBytes = info.Size()
	result.Reduced = result.OutputBytes < result.InputBytes
	if !result.Reduced {
		candidate = snapshot
		result.OutputBytes = result.InputBytes
	}
	if err = os.Chmod(candidate, 0o600); err != nil {
		return result, errors.New("pdf: cannot secure output permissions")
	}
	if ctx.Err() != nil {
		return result, fmt.Errorf("pdf: compression interrupted: %w", ctx.Err())
	}
	// Prefer atomic publication. Filesystems without hard links use an exclusive copy,
	// with cleanup on cancellation or failure and no replacement of an existing destination.
	if err = publishPDF(ctx, candidate, output, os.Link); err != nil {
		return result, err
	}
	return result, nil
}

func copySnapshot(name string, src io.Reader) (int64, error) {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, errors.New("pdf: cannot create input snapshot")
	}
	n, copyErr := io.Copy(f, io.LimitReader(src, maxInputBytes+1))
	closeErr := f.Close()
	if copyErr != nil {
		return 0, fmt.Errorf("pdf: cannot copy input: %w", copyErr)
	}
	if closeErr != nil {
		return 0, errors.New("pdf: cannot close input snapshot")
	}
	if n > maxInputBytes {
		return 0, errors.New("pdf: input exceeds 1 GiB")
	}
	f, err = os.Open(name)
	if err != nil {
		return 0, errors.New("pdf: cannot inspect input")
	}
	defer func() { _ = f.Close() }()
	header := make([]byte, 1024)
	count, readErr := f.Read(header)
	if (readErr != nil && !errors.Is(readErr, io.EOF)) || !bytes.Contains(header[:count], []byte("%PDF-")) {
		return 0, errors.New("pdf: input is not a PDF")
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return 0, errors.New("pdf: cannot inspect protection")
	}
	if err = rejectEncryption(f); err != nil {
		return 0, err
	}
	return n, nil
}

func compressionArgs(quality Quality) []string {
	args := []string{"-q", "-dSAFER", "-dBATCH", "-dNOPAUSE", "-dPDFSTOPONERROR", "-sDEVICE=pdfwrite", "-dCompatibilityLevel=1.7", "-dAutoRotatePages=/None", "-dCompressPages=true", "-dCompressFonts=true", "-dCompressStreams=true", "-dSubsetFonts=true", "-dEmbedAllFonts=true", "-dDetectDuplicateImages=true", "-sOutputFile=compressed.pdf"}
	if quality == High {
		args = append(args, "-dDownsampleColorImages=false", "-dDownsampleGrayImages=false", "-dDownsampleMonoImages=false")
	} else {
		dpi := "150"
		if quality == Small {
			dpi = "96"
		}
		args = append(args, "-dDownsampleColorImages=true", "-dDownsampleGrayImages=true", "-dDownsampleMonoImages=true", "-dColorImageResolution="+dpi, "-dGrayImageResolution="+dpi, "-dMonoImageResolution=300", "-dColorImageDownsampleType=/Bicubic", "-dGrayImageDownsampleType=/Bicubic", "-dMonoImageDownsampleType=/Subsample")
	}
	return append(args, "-f", "input.pdf")
}

func pageCount(ctx context.Context, engine, dir, name string) (int, error) {
	cmd := exec.CommandContext(ctx, engine, "-q", "-dSAFER", "-dBATCH", "-dNOPAUSE", "-dPDFSTOPONERROR", "-dNODISPLAY", "--permit-file-read="+name, "-sPDFPath="+name, "-c", "PDFPath (r) file runpdfbegin pdfpagecount = quit") //nolint:gosec // Fixed filenames and PostScript; no input data is evaluated as code.
	cmd.Dir = dir
	output, err := runEngine(ctx, cmd)
	if err != nil {
		return 0, err
	}
	count, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil || count <= 0 {
		return 0, errors.New("pdf: invalid PDF page count")
	}
	return count, nil
}

// rejectEncryption conservatively scans PDF names, including #xx escapes. The
// Encrypt name in a trailer/xref dictionary must be outside encrypted streams.
// Rejecting this marker also in content is intentional: this is not a full PDF parser.
func rejectEncryption(input io.Reader) error {
	r := bufio.NewReader(input)
	for {
		b, err := r.ReadByte()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return errors.New("pdf: cannot inspect protection")
		}
		if b != '/' {
			continue
		}
		var name strings.Builder
		for {
			b, err = r.ReadByte()
			if err != nil && !errors.Is(err, io.EOF) {
				return errors.New("pdf: cannot inspect protection")
			}
			if err != nil || strings.ContainsRune("\x00\t\n\f\r ()<>[]{}/%", rune(b)) {
				if err == nil {
					if err = r.UnreadByte(); err != nil {
						return err
					}
				}
				break
			}
			if name.Len() < 256 {
				name.WriteByte(b)
			}
		}
		raw := name.String()
		var decoded strings.Builder
		for i := 0; i < len(raw); i++ {
			if raw[i] == '#' && i+2 < len(raw) {
				v, e := strconv.ParseUint(raw[i+1:i+3], 16, 8)
				if e == nil {
					decoded.WriteByte(byte(v))
					i += 2
					continue
				}
			}
			decoded.WriteByte(raw[i])
		}
		if decoded.String() == "Encrypt" {
			return errors.New("pdf: encrypted or permission-restricted PDFs are not supported; compression would remove protection")
		}
	}
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

func publishPDF(ctx context.Context, source, output string, link func(string, string) error) error {
	if err := link(source, output); err == nil {
		f, err := os.OpenFile(output, os.O_RDWR, 0)
		if err == nil {
			err = errors.Join(f.Sync(), f.Close())
		}
		if err == nil {
			err = syncDir(filepath.Dir(output))
		}
		if err != nil {
			_ = os.Remove(output)
			return fmt.Errorf("pdf: cannot flush published output: %w", err)
		}
		return nil
	}
	in, err := os.Open(source)
	if err != nil {
		return errors.New("pdf: cannot read compressed file")
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return errors.New("pdf: cannot publish output; destination must be new")
	}
	ok := false
	defer func() {
		_ = out.Close()
		if !ok {
			_ = os.Remove(output)
		}
	}()
	if _, err = io.Copy(out, contextReader{ctx, in}); err != nil {
		return fmt.Errorf("pdf: publishing interrupted: %w", err)
	}
	if err = out.Sync(); err != nil {
		return errors.New("pdf: cannot flush output")
	}
	if err = out.Close(); err != nil {
		return errors.New("pdf: cannot close output")
	}
	if err = ctx.Err(); err != nil {
		return fmt.Errorf("pdf: publishing interrupted: %w", err)
	}
	if err = syncDir(filepath.Dir(output)); err != nil {
		return fmt.Errorf("pdf: cannot flush output directory: %w", err)
	}
	ok = true
	return nil
}
