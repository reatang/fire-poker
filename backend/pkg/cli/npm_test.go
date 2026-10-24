package cli

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Uses an actual Node child process with an offline npm stub, including the
// Windows npm.cmd layout, to verify filesystem writes without installing packages.
func TestNpmInstallPrivateWrites(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node is required for the offline child-process integration test")
	}
	for _, windowsWrapper := range []bool{false, true} {
		t.Run(map[bool]string{false: "script", true: "windows-wrapper"}[windowsWrapper], func(t *testing.T) {
			root := t.TempDir()
			target := filepath.Join(root, "install")
			private := filepath.Join(root, "private")
			host := filepath.Join(root, "host")
			if err := os.MkdirAll(host, 0o700); err != nil {
				t.Fatal(err)
			}
			npmPath := filepath.Join(root, "npm.js")
			scriptPath := npmPath
			if windowsWrapper {
				npmPath = filepath.Join(root, "npm.cmd")
				scriptPath = filepath.Join(root, "node_modules", "npm", "bin", "npm-cli.js")
				if err := os.MkdirAll(filepath.Dir(scriptPath), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(npmPath, []byte("@echo off\r\nexit /b 99\r\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			script := `const fs = require('fs'); const path = require('path');
const keys = ['HOME','USERPROFILE','APPDATA','LOCALAPPDATA','TEMP','TMP','TMPDIR','NPM_CONFIG_CACHE','NPM_CONFIG_LOGS_DIR'];
const values = Object.fromEntries(keys.map(k => [k, process.env[k]]));
for (const dir of Object.values(values)) fs.writeFileSync(path.join(dir, 'child-marker'), 'private');
fs.writeFileSync(path.join(process.cwd(), 'observed.json'), JSON.stringify({values, argv: process.argv.slice(2)}));`
			if err := os.WriteFile(scriptPath, []byte(script), 0o600); err != nil {
				t.Fatal(err)
			}
			parent := append(os.Environ(), "HOME="+host, "APPDATA="+host, "TEMP="+host, "npm_config_cache="+host)
			if err := NpmInstall(context.Background(), NpmInstallParams{NodePath: node, NpmPath: npmPath, Source: "offline-fixture", TargetDir: target, CLIDataDir: private, ParentEnv: parent}); err != nil {
				t.Fatal(err)
			}
			bytes, err := os.ReadFile(filepath.Join(target, "observed.json"))
			if err != nil {
				t.Fatal(err)
			}
			var observed struct {
				Values map[string]string
				Argv   []string
			}
			if err := json.Unmarshal(bytes, &observed); err != nil {
				t.Fatal(err)
			}
			for key, value := range observed.Values {
				rel, err := filepath.Rel(private, value)
				if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					t.Fatalf("%s escaped: %q", key, value)
				}
				if _, err := os.Stat(filepath.Join(value, "child-marker")); err != nil {
					t.Fatalf("%s write failed: %v", key, err)
				}
			}
			if !strings.Contains(strings.Join(observed.Argv, " "), "--prefix "+target) {
				t.Fatalf("install prefix missing: %v", observed.Argv)
			}
			entries, err := os.ReadDir(host)
			if err != nil || len(entries) != 0 {
				t.Fatalf("host directory changed: %v (%v)", entries, err)
			}
		})
	}
}

// writeFakeNpm writes a fake bundled node + npm pair that records every invocation into log files.
// The fake node accepts "node <npmPath> <args...>", creates the --prefix dir if requested, and exits 0.
func writeFakeNpm(t *testing.T, recordDir string) (nodePath, npmPath string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("npm install tests use POSIX shell mocks")
	}
	binDir := t.TempDir()
	npmPath = filepath.Join(binDir, "npm")
	nodePath = filepath.Join(binDir, "node")
	if err := os.WriteFile(npmPath, []byte("// fake npm entry\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	nodeBody := `#!/usr/bin/env bash
echo "NODE:$@" >> "` + filepath.Join(recordDir, "node.log") + `"
echo "SCRIPT:$1" >> "` + filepath.Join(recordDir, "npm.log") + `"
shift
echo "ARGS:$@" >> "` + filepath.Join(recordDir, "npm.log") + `"
i=0
for a in "$@"; do
  i=$((i+1))
  if [ "$a" = "--prefix" ]; then
    eval "next=\${$((i+1))}"
    mkdir -p "$next/node_modules/.bin"
  fi
done
exit 0
`
	if err := os.WriteFile(nodePath, []byte(nodeBody), 0o755); err != nil {
		t.Fatal(err)
	}
	return nodePath, npmPath
}

// TestNpmInstallPassesPrefix verifies the install call includes --prefix <targetDir>.
func TestNpmInstallPassesPrefix(t *testing.T) {
	logDir := t.TempDir()
	nodePath, npmPath := writeFakeNpm(t, logDir)
	targetDir := filepath.Join(t.TempDir(), "install")

	err := NpmInstall(context.Background(), NpmInstallParams{
		NodePath:  nodePath,
		NpmPath:   npmPath,
		Source:    "lark-cli",
		TargetDir: targetDir,
		ParentEnv: []string{"PATH=/usr/bin:/bin"},
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	logBytes, err := os.ReadFile(filepath.Join(logDir, "npm.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logBytes), "--prefix") || !strings.Contains(string(logBytes), targetDir) {
		t.Fatalf("expected --prefix %s in log, got %q", targetDir, string(logBytes))
	}
	if !strings.Contains(string(logBytes), "lark-cli") {
		t.Fatalf("expected source 'lark-cli' in log, got %q", string(logBytes))
	}
	if !strings.Contains(string(logBytes), "--no-audit") || !strings.Contains(string(logBytes), "--no-fund") {
		t.Fatalf("expected --no-audit and --no-fund flags, got %q", string(logBytes))
	}

	nodeLogBytes, err := os.ReadFile(filepath.Join(logDir, "node.log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(nodeLogBytes), npmPath) {
		t.Fatalf("expected bundled node to receive npm path %q, got %q", npmPath, string(nodeLogBytes))
	}
}

// TestNpmInstallStripsNpmConfigPrefix verifies the inherited NPM_CONFIG_PREFIX is removed before invoking npm.
func TestNpmInstallStripsNpmConfigPrefix(t *testing.T) {
	logDir := t.TempDir()
	nodePath, npmPath := writeFakeNpm(t, logDir)
	targetDir := filepath.Join(t.TempDir(), "install")

	envInspectPath := filepath.Join(filepath.Dir(npmPath), "envcheck")
	envCheckBody := `#!/usr/bin/env bash
env | grep -E '^NPM_CONFIG_PREFIX=' > "` + filepath.Join(logDir, "env.log") + `" || true
exit 0
`
	if err := os.WriteFile(envInspectPath, []byte(envCheckBody), 0o755); err != nil {
		t.Fatal(err)
	}
	nodeInspectBody := `#!/usr/bin/env bash
script="$1"
shift
exec "$script" "$@"
`
	if err := os.WriteFile(nodePath, []byte(nodeInspectBody), 0o755); err != nil {
		t.Fatal(err)
	}

	err := NpmInstall(context.Background(), NpmInstallParams{
		NodePath:  nodePath,
		NpmPath:   envInspectPath, // use env-inspecting fake instead
		Source:    "x",
		TargetDir: targetDir,
		ParentEnv: []string{"PATH=/usr/bin:/bin", "NPM_CONFIG_PREFIX=/old/leaks"},
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	envLog, _ := os.ReadFile(filepath.Join(logDir, "env.log"))
	if len(envLog) != 0 {
		t.Fatalf("NPM_CONFIG_PREFIX should have been stripped, got: %q", string(envLog))
	}
}
