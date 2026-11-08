//go:build !unix

package cdp

import "os/exec"

func configureBrowserProcess(cmd *exec.Cmd) {}
