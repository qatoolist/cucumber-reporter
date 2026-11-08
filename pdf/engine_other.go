//go:build !unix

package pdf

import "os/exec"

func configureProcess(cmd *exec.Cmd) {}
