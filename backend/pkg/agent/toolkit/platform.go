package toolkit

import "runtime"

const (
	WindowsShellTool = "windows_shell"
	MacOSShellTool   = "macos_shell"
	LinuxShellTool   = "linux_shell"
)

// PlatformShellToolID returns the only shell capability supported by this host.
func PlatformShellToolID() string {
	switch runtime.GOOS {
	case "windows":
		return WindowsShellTool
	case "darwin":
		return MacOSShellTool
	case "linux":
		return LinuxShellTool
	default:
		return ""
	}
}

func IsShellTool(name string) bool {
	return name == "shell" || name == WindowsShellTool || name == MacOSShellTool || name == LinuxShellTool
}
