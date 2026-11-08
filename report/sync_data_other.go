//go:build !darwin

package report

import "os"

func syncFileData(file *os.File) error { return file.Sync() }
