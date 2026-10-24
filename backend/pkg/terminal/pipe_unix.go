//go:build !windows

package terminal

import "os/exec"

func configurePipeCommand(cmd *exec.Cmd) {}
