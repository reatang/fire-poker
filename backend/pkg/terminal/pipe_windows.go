//go:build windows

package terminal

import (
	"os/exec"
	"syscall"
)

func configurePipeCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}
