package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/reatang/fire-poker/backend/pkg/agent/toolkit"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

func TestPlatformShellResolution(t *testing.T) {
	modernPath := filepath.Join("programs", "PowerShell", "7", "pwsh.exe")
	legacyPath := filepath.Join("windows", "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	for _, tc := range []struct {
		name, goos string
		installed  []string
		want       string
	}{
		{"windows_prefers_pwsh", "windows", []string{"pwsh.exe", "powershell.exe"}, "pwsh.exe"},
		{"windows_probes_install_dir_before_legacy", "windows", []string{modernPath, "powershell.exe"}, modernPath},
		{"windows_legacy_path", "windows", []string{"powershell.exe"}, "powershell.exe"},
		{"windows_legacy_install_dir", "windows", []string{legacyPath}, legacyPath},
		{"windows_never_uses_cmd_or_bash", "windows", []string{"cmd.exe", "bash"}, "pwsh.exe"},
		{"mac_prefers_zsh", "darwin", []string{"zsh", "bash", "sh", "pwsh.exe"}, "zsh"},
		{"mac_falls_back_to_bash", "darwin", []string{"bash", "sh"}, "bash"},
		{"linux_prefers_bash", "linux", []string{"zsh", "bash", "sh", "pwsh.exe"}, "bash"},
		{"linux_falls_back_to_sh", "linux", []string{"sh"}, "sh"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			getenv := func(key string) string {
				switch key {
				case "ProgramFiles":
					return "programs"
				case "SystemRoot":
					return "windows"
				}
				return ""
			}
			lookup := func(name string) (string, error) {
				for _, installed := range tc.installed {
					if name == installed {
						return name, nil
					}
				}
				return "", fmt.Errorf("not installed: %s", name)
			}
			shell, args := resolvePlatformShellCommand(tc.goos, "original command", getenv, lookup)
			wantArgs := []string{"-lc", "original command"}
			if tc.goos == "windows" {
				wantArgs = []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-Command", "original command"}
			}
			if shell != tc.want || !reflect.DeepEqual(args, wantArgs) {
				t.Fatalf("got %q %v; want %q %v", shell, args, tc.want, wantArgs)
			}
		})
	}
}

func TestForeignPlatformShellsCannotRegisterOrExecute(t *testing.T) {
	r := toolkit.NewRegistry()
	if err := RegisterDefaults(r, nil); err != nil {
		t.Fatal(err)
	}
	ids := []string{toolkit.WindowsShellTool, toolkit.MacOSShellTool, toolkit.LinuxShellTool}
	built, err := r.BuildTools(toolkit.Scope{EnabledTools: ids})
	if err != nil {
		t.Fatal(err)
	}
	if len(built) != 1 || built[0].Declaration().Name != toolkit.PlatformShellToolID() {
		t.Fatalf("foreign shell registered: %v", built)
	}
	for _, meta := range r.ListedTools() {
		if toolkit.IsShellTool(meta.Name) && meta.Name != toolkit.PlatformShellToolID() {
			t.Fatalf("foreign shell listed: %s", meta.Name)
		}
	}
	for _, platform := range []string{"windows", "darwin", "linux"} {
		if platform == runtime.GOOS {
			continue
		}
		t.Run(platform, func(t *testing.T) {
			runner := &fakeTerminalRunner{}
			foreign := newPlatformShellTool(platform, nil, 0, "", runner)
			_, err := tool.CallableTool(foreign).Call(context.Background(), []byte(`{"command":"echo forbidden"}`))
			if err == nil || !strings.Contains(err.Error(), "disabled") {
				t.Fatalf("foreign execution was not rejected: %v", err)
			}
			if runner.createParams.Command != "" {
				t.Fatal("foreign shell started a terminal")
			}
		})
	}
}

func TestShellFuncPreservesNonZeroExitCode(t *testing.T) {
	out, err := shellFunc(context.Background(), nil, 0, shellInput{Command: "exit 7"})
	if err != nil || out.ExitCode != 7 {
		t.Fatalf("exit=%d err=%v", out.ExitCode, err)
	}
}
