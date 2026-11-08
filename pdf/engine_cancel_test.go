//go:build unix

package pdf

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestEngineCancellationKillsWrapperChildren(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := runEngine(ctx, exec.CommandContext(ctx, "sh", "-c", "sleep 20 & wait"))
	if err == nil {
		t.Fatal("expected cancellation")
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("cancellation took %s", elapsed)
	}
}
