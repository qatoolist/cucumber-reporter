//go:build windows

package report

// Windows does not expose directory fsync through os.File. Each created file is
// flushed before publication; rename uses the platform filesystem semantics.
func syncDir(path string) error { return nil }
