package report

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

// Compare the pre-durability cost with serial and bounded concurrent flushing.
// Timings are diagnostic, not a brittle filesystem-dependent pass/fail threshold.
func BenchmarkPublish3000Sidecars(b *testing.B) {
	for _, mode := range []string{"write-only", "serial-sync", "batched-sync"} {
		b.Run(mode, func(b *testing.B) {
			data := make([]byte, 4096)
			for i := 0; i < b.N; i++ {
				dir := b.TempDir()
				for j := 0; j < 3000; j++ {
					path := filepath.Join(dir, fmt.Sprintf("attachment-%d.png", j))
					if err := writeNew(path, data); err != nil {
						b.Fatal(err)
					}
					if mode == "serial-sync" {
						f, err := os.OpenFile(path, os.O_RDWR, 0)
						if err != nil {
							b.Fatal(err)
						}
						err = f.Sync()
						closeErr := f.Close()
						if err != nil || closeErr != nil {
							b.Fatal(err, closeErr)
						}
					}
				}
				if mode == "batched-sync" {
					if err := syncFiles(dir); err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}

// A release benchmark compares like-for-like publication on the same filesystem.
// Median samples reduce cache and scheduling noise; the budget is explicit.
func BenchmarkSidecarRelativeBudget(b *testing.B) {
	for n := 0; n < b.N; n++ {
		var baseline, durable []time.Duration
		for trial := 0; trial < 3; trial++ {
			for _, sync := range []bool{false, true} {
				dir := b.TempDir()
				data := make([]byte, 4096)
				start := time.Now()
				write := writeNew
				var flusher *fileFlusher
				if sync {
					flusher = newFileFlusher(syncFileData)
					defer func() { _ = flusher.finish() }()
					write = flusher.write
				}
				for i := 0; i < 3000; i++ {
					data[0] = byte(i)
					data[1] = byte(i >> 8)
					if e := write(filepath.Join(dir, fmt.Sprintf("evidence-%d.bin", i)), data); e != nil {
						b.Fatal(e)
					}
				}
				if sync {
					if e := errors.Join(flusher.finish(), syncDir(dir)); e != nil {
						b.Fatal(e)
					}
					durable = append(durable, time.Since(start))
				} else {
					baseline = append(baseline, time.Since(start))
				}
			}
		}
		slices.Sort(baseline)
		slices.Sort(durable)
		ratio := float64(durable[1]) / float64(baseline[1])
		b.ReportMetric(ratio, "durable/write-only")
		if ratio > 2 {
			b.Fatalf("3000 sidecars exceed 2x publication budget: write-only %s, durable %s (%.2fx)", baseline[1], durable[1], ratio)
		}
	}
}

func TestSyncFilesCompletesDataBeforePublicationBarrier(t *testing.T) {
	for _, fail := range []bool{false, true} {
		dir := t.TempDir()
		for i := 0; i < 32; i++ {
			if err := writeNew(filepath.Join(dir, fmt.Sprint(i)), []byte("evidence")); err != nil {
				t.Fatal(err)
			}
		}
		var completed atomic.Int32
		sentinel := errors.New("storage failure")
		var barrierCalled bool
		err := syncFilesWith(dir, func(f *os.File) error {
			if _, err := f.Stat(); err != nil {
				return err
			}
			completed.Add(1)
			if fail && filepath.Base(f.Name()) == "0" {
				return sentinel
			}
			return nil
		}, func(string) error {
			barrierCalled = true
			if completed.Load() != 32 {
				t.Fatal("barrier preceded file completion")
			}
			return nil
		})
		if fail {
			if !errors.Is(err, sentinel) || barrierCalled {
				t.Fatalf("failed data flush published: %v %v", err, barrierCalled)
			}
		} else if err != nil || !barrierCalled {
			t.Fatalf("publication barrier missing: %v", err)
		}
	}
	dir := t.TempDir()
	sentinel := errors.New("barrier failure")
	if err := syncFilesWith(dir, func(*os.File) error { return nil }, func(string) error { return sentinel }); !errors.Is(err, sentinel) {
		t.Fatalf("barrier failure lost: %v", err)
	}
}

func TestSyncFileDataClosedHandle(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "closed-")
	if err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	if err = syncFileData(f); err == nil {
		t.Fatalf("closed file accepted: %v", err)
	}
}

func TestFileFlusherWaitsClosesAndPropagatesFailures(t *testing.T) {
	for _, fail := range []bool{false, true} {
		sentinel := errors.New("storage flush failure")
		var completed atomic.Int32
		flusher := newFileFlusher(func(file *os.File) error {
			completed.Add(1)
			if fail && filepath.Base(file.Name()) == "0" {
				return sentinel
			}
			return syncFileData(file)
		})
		defer func() { _ = flusher.finish() }()
		dir := t.TempDir()
		var files []*os.File
		for i := 0; i < 100; i++ {
			file, err := os.OpenFile(filepath.Join(dir, fmt.Sprint(i)), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = file.WriteString("retained evidence"); err != nil {
				t.Fatal(err)
			}
			files = append(files, file)
			flusher.queue(file)
		}
		err := flusher.finish()
		if fail && !errors.Is(err, sentinel) || !fail && err != nil {
			t.Fatalf("flush result: %v", err)
		}
		if completed.Load() != 100 {
			t.Fatalf("unfinished flushes: %d", completed.Load())
		}
		if !errors.Is(flusher.finish(), err) {
			t.Fatal("finish is not idempotent")
		}
		for _, file := range files {
			if _, err := file.WriteString("late write"); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("handle retained: %v", err)
			}
			data, err := os.ReadFile(file.Name())
			if err != nil || string(data) != "retained evidence" {
				t.Fatalf("evidence changed: %q %v", data, err)
			}
		}
	}
}
