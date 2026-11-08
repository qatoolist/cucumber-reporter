package pdf

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type cappedOutput struct {
	data []byte
	tail []byte
}

func (b *cappedOutput) Write(p []byte) (int, error) {
	n := len(p)
	b.tail = append(b.tail, p...)
	if len(b.tail) > 4096 {
		b.tail = append([]byte(nil), b.tail[len(b.tail)-4096:]...)
	}
	remaining := 4096 - len(b.data)
	if remaining > 0 {
		b.data = append(b.data, p[:min(remaining, n)]...)
	}
	return n, nil
}

func discoverEngine(selected string, lookup func(string) (string, error)) (string, error) {
	names := []string{"gs", "gswin64c", "gswin32c"}
	if selected != "" {
		names = []string{selected}
	}
	for _, name := range names {
		if path, err := lookup(name); err == nil {
			return path, nil
		}
	}
	return "", errors.New("pdf: Ghostscript is required for compression; install it or set GS to its executable")
}

func runEngine(ctx context.Context, cmd *exec.Cmd) ([]byte, error) {
	cmd.WaitDelay = 2 * time.Second
	configureProcess(cmd)
	var out, stderr cappedOutput
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("pdf: Ghostscript interrupted: %w", ctx.Err())
		}
		if errors.Is(err, exec.ErrWaitDelay) {
			return nil, errors.New("pdf: Ghostscript exited but a child kept its output pipes open beyond the cleanup deadline")
		}
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			hint := "check installation, available disk space and PDF validity"
			if strings.Contains(strings.ToLower(string(stderr.data)+string(stderr.tail)), "no space left") {
				hint = "output disk is full"
			}
			if exit.ExitCode() < 0 {
				return nil, fmt.Errorf("pdf: Ghostscript terminated by signal; %s", hint)
			}
			return nil, fmt.Errorf("pdf: Ghostscript failed (exit %d); %s", exit.ExitCode(), hint)
		}
		return nil, errors.New("pdf: Ghostscript could not start; check executable permissions")
	}
	return out.data, nil
}

func checkEngineVersion(ctx context.Context, engine string) error {
	out, err := runEngine(ctx, exec.CommandContext(ctx, engine, "--version")) //nolint:gosec // Operator-selected executable and fixed argument.
	if err != nil {
		return err
	}
	parts := strings.Split(strings.TrimSpace(string(out)), ".")
	if len(parts) < 2 {
		return errors.New("pdf: cannot determine Ghostscript version; version 9.50 or newer is required")
	}
	major, e1 := strconv.Atoi(parts[0])
	minor, e2 := strconv.Atoi(parts[1])
	if e1 != nil || e2 != nil || major < 9 || (major == 9 && minor < 50) {
		return errors.New("pdf: Ghostscript 9.50 or newer is required; upgrade the installed executable")
	}
	return nil
}
