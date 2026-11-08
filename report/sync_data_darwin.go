package report

import (
	"errors"
	"os"
	"syscall"
)

// Flush each file to the device, then syncFiles calls syncDir once to flush the
// drive cache. Go's File.Sync uses F_FULLFSYNC on Darwin; doing that per sidecar
// repeatedly flushes the whole drive. No report is published before the final
// full barrier. See Apple's fsync(2) and fcntl(2) documentation:
// https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/fsync.2.html
func syncFileData(file *os.File) error {
	conn, err := file.SyscallConn()
	if err != nil {
		return err
	}
	var syncErr error
	err = conn.Control(func(fd uintptr) {
		for {
			syncErr = syscall.Fsync(int(fd))
			if !errors.Is(syncErr, syscall.EINTR) {
				break
			}
		}
	})
	return errors.Join(err, syncErr)
}
