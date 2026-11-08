package report

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// Flush all closed files before publishing the directory. A bounded pool avoids
// serial fsync latency on attachment-heavy reports without unbounded open files.
// The final directory sync is also the full device-cache barrier on macOS.
func syncFiles(dir string) error {
	return syncFilesWith(dir, syncFileData, syncDir)
}

func syncFilesWith(dir string, syncData func(*os.File) error, barrier func(string) error) error {
	jobs := make(chan string)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var first error
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				f, err := os.OpenFile(path, os.O_RDWR, 0)
				if err == nil {
					err = errors.Join(syncData(f), f.Close())
				}
				if err != nil {
					mu.Lock()
					if first == nil {
						first = err
					}
					mu.Unlock()
				}
			}
		}()
	}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			jobs <- path
		}
		return nil
	})
	close(jobs)
	wg.Wait()
	if err = errors.Join(err, first); err != nil {
		return err
	}
	return barrier(dir)
}

// fileFlusher overlaps file-data flushes with writes while retaining a bounded
// number of handles. The caller must sync the directory after finish succeeds;
// on Darwin that is the full device-cache barrier for the whole publication.
type fileFlusher struct {
	jobs chan *os.File
	wg   sync.WaitGroup
	once sync.Once
	mu   sync.Mutex
	err  error
}

func newFileFlusher(syncData func(*os.File) error) *fileFlusher {
	f := &fileFlusher{jobs: make(chan *os.File, 32)}
	for i := 0; i < 32; i++ {
		f.wg.Add(1)
		go func() {
			defer f.wg.Done()
			for file := range f.jobs {
				if err := errors.Join(syncData(file), file.Close()); err != nil {
					f.mu.Lock()
					if f.err == nil {
						f.err = err
					}
					f.mu.Unlock()
				}
			}
		}()
	}
	return f
}
func (f *fileFlusher) queue(file *os.File) { f.jobs <- file }
func (f *fileFlusher) write(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(data); err != nil {
		return errors.Join(err, file.Close())
	}
	f.queue(file)
	return nil
}

func (f *fileFlusher) finish() error {
	f.once.Do(func() { close(f.jobs); f.wg.Wait() })
	return f.err
}
