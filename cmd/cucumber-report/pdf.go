package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/qatoolist/cucumber-reporter/pdf"
)

const pdfUsage = "usage: cucumber-report compress-pdf [-quality high|balanced|small] [-o output.pdf] [-timeout 5m] input.pdf\n"

func runCompressPDF(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("compress-pdf", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	quality := fs.String("quality", "balanced", "high: no image downsampling; balanced: 150 dpi; small: 96 dpi")
	timeout := fs.Duration("timeout", 5*time.Minute, "maximum compression time")
	output := fs.String("o", "", "new output PDF (default: input name with .shared.pdf replacing its extension)")
	args = interspersedPDFArgs(args)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(stdout, pdfUsage)
			return 0
		}
		fmt.Fprintf(stderr, "cucumber-report: %v\n%s", err, pdfUsage)
		return 2
	}
	problem := ""
	if fs.NArg() != 1 {
		problem = "exactly one input PDF is required"
	} else if *timeout <= 0 {
		problem = "-timeout must be greater than zero"
	} else if *quality != "high" && *quality != "balanced" && *quality != "small" {
		problem = "-quality must be high, balanced or small"
	}
	if problem != "" {
		fmt.Fprintf(stderr, "cucumber-report: %s\n%s", problem, pdfUsage)
		return 2
	}
	input := fs.Arg(0)
	if *output == "" {
		*output = strings.TrimSuffix(input, filepath.Ext(input)) + ".shared.pdf"
	}
	signals, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(signals, *timeout)
	defer cancel()
	result, err := pdf.Compress(ctx, input, *output, pdf.Options{Quality: pdf.Quality(*quality)})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			err = fmt.Errorf("PDF compression timed out after %s; raise -timeout", *timeout)
		}
		_, _ = fmt.Fprintf(stderr, "cucumber-report: %v\n", err)
		if signals.Err() != nil {
			return 130
		}
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "PDF: %s\nPages: %d (all retained)\nSize: %.2f MiB (%d bytes) -> %.2f MiB (%d bytes) (%.1f%% smaller)\nQuality: %s\n", *output, result.Pages, float64(result.InputBytes)/(1<<20), result.InputBytes, float64(result.OutputBytes)/(1<<20), result.OutputBytes, 100*(1-float64(result.OutputBytes)/float64(result.InputBytes)), *quality)
	if !result.Reduced {
		_, _ = fmt.Fprintln(stdout, "Already compact: saved an unchanged copy because recompression was not smaller.")
	}
	return 0
}

// flag.FlagSet stops at a positional argument; accept flags on either side,
// retaining -- as the explicit boundary for filenames beginning with a dash.
func interspersedPDFArgs(args []string) []string {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
			flagName := strings.TrimLeft(a, "-")
			if flagName == "quality" || flagName == "o" || flagName == "timeout" {
				if i+1 >= len(args) {
					return flags
				}
				i++
				flags = append(flags, args[i])
			}
		} else {
			pos = append(pos, a)
		}
	}
	return append(append(flags, "--"), pos...)
}
