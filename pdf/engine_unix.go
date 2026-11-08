//go:build unix

package pdf

import (
	"os/exec"
	"syscall"
)

// A wrapper may leave children holding the output pipes after its parent exits.
func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}
