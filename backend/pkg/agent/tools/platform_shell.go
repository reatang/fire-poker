package tools

import (
	"github.com/reatang/fire-poker/backend/pkg/agent/toolkit"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

func platformShellName(goos string) string {
	switch goos {
	case "windows":
		return toolkit.WindowsShellTool
	case "darwin":
		return toolkit.MacOSShellTool
	case "linux":
		return toolkit.LinuxShellTool
	default:
		return ""
	}
}

func platformShellDescription(goos string) string {
	switch goos {
	case "windows":
		return "Execute Windows PowerShell commands. Prefer installed PowerShell 7 (pwsh), then Windows PowerShell. Use PowerShell syntax, not cmd.exe or POSIX shell syntax. This tool is disabled on non-Windows systems."
	case "darwin":
		return "Execute macOS shell commands using zsh, falling back to bash or sh. Use macOS-compatible commands and POSIX shell syntax. This tool is disabled on non-macOS systems."
	case "linux":
		return "Execute Linux shell commands using bash, falling back to sh. Use Linux-compatible commands and POSIX shell syntax. This tool is disabled on non-Linux systems."
	default:
		return "Shell execution is disabled on this operating system."
	}
}

func NewWindowsShellTool(emitter ShellProgressEmitter, sessionID uint, runner ...ShellTerminalRunner) *function.FunctionTool[shellInput, shellOutput] {
	return newPlatformShellTool("windows", emitter, sessionID, runner...)
}

func NewMacOSShellTool(emitter ShellProgressEmitter, sessionID uint, runner ...ShellTerminalRunner) *function.FunctionTool[shellInput, shellOutput] {
	return newPlatformShellTool("darwin", emitter, sessionID, runner...)
}

func NewLinuxShellTool(emitter ShellProgressEmitter, sessionID uint, runner ...ShellTerminalRunner) *function.FunctionTool[shellInput, shellOutput] {
	return newPlatformShellTool("linux", emitter, sessionID, runner...)
}
