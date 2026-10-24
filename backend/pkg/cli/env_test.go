package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPrepareEnvPrivateDirectories(t *testing.T) {
	root := filepath.Join(t.TempDir(), "private")
	parent := []string{"PATH=/system/bin", "APPDATA=/host/roaming", "LOCALAPPDATA=/host/local", "TEMP=/host/tmp", "TMP=/host/tmp", "TMPDIR=/host/tmp", "npm_config_cache=/host/npm", "NPM_CONFIG_LOGS_DIR=/host/logs", "npm_config_userconfig=/host/npmrc", "npm_config_globalconfig=/host/global", "npm_config_prefix=/host/prefix", "HTTPS_PROXY=http://proxy.invalid"}
	if runtime.GOOS == "windows" {
		parent[0] = "Path=C:\\Windows"
		parent = append(parent, "AppData=C:\\host", "UserProfile=C:\\host", "Temp=C:\\host")
	}
	env, err := PrepareEnv(EnvParams{CLIDataDir: root, ParentEnv: parent})
	if err != nil {
		t.Fatal(err)
	}
	got := envMap(env)
	for _, key := range []string{"HOME", "USERPROFILE", "APPDATA", "LOCALAPPDATA", "TEMP", "TMP", "TMPDIR", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME", "NPM_CONFIG_CACHE", "NPM_CONFIG_LOGS_DIR"} {
		value := got[key]
		rel, err := filepath.Rel(root, value)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatalf("%s escaped private root: %q", key, value)
		}
		if info, err := os.Stat(value); err != nil || !info.IsDir() {
			t.Fatalf("%s directory not prepared: %q (%v)", key, value, err)
		}
	}
	for _, entry := range env {
		if strings.HasPrefix(strings.ToLower(entry), "npm_config_prefix=") || strings.Contains(entry, "/host/") || strings.Contains(entry, "C:\\host") {
			t.Fatalf("host directory leaked: %q", entry)
		}
	}
	if got["HTTPS_PROXY"] != "http://proxy.invalid" {
		t.Fatal("network configuration must remain available")
	}
	if _, err := PrepareEnv(EnvParams{CLIDataDir: "relative"}); err == nil {
		t.Fatal("relative private roots must be rejected")
	}
}

func TestPrepareEnvSharedDoesNotCreatePrivateDirectories(t *testing.T) {
	root := filepath.Join(t.TempDir(), "unused")
	env, err := PrepareEnv(EnvParams{Isolation: IsolationShared, CLIDataDir: root, ParentEnv: []string{"APPDATA=/user/config", "TEMP=/user/tmp"}})
	if err != nil {
		t.Fatal(err)
	}
	if envMap(env)["APPDATA"] != "/user/config" || envMap(env)["TEMP"] != "/user/tmp" {
		t.Fatal("explicit shared mode must preserve host directories")
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("shared mode unexpectedly created private directories: %v", err)
	}
}

// TestBuildEnvIsolatedSetsHomeAndXDG verifies isolated mode injects HOME / XDG_* pointing into the plugin data dir.
func TestBuildEnvIsolatedSetsHomeAndXDG(t *testing.T) {
	dataDir := t.TempDir()
	cliDataRoot := filepath.Join(dataDir, "plugins", "cli_data", "lark-cli")

	env := BuildEnv(EnvParams{
		Isolation:  IsolationIsolated,
		CLIDataDir: cliDataRoot,
		PluginBin:  filepath.Join(dataDir, "plugins", "cli", "lark-cli", "node_modules", ".bin"),
		ParentEnv:  []string{"PATH=/usr/bin", "HOME=/Users/me", "NPM_CONFIG_PREFIX=/old", "FOO=BAR"},
	})

	got := envMap(env)
	if got["HOME"] != filepath.Join(cliDataRoot, "home") {
		t.Fatalf("HOME=%q", got["HOME"])
	}
	if got["XDG_CONFIG_HOME"] != filepath.Join(cliDataRoot, "config") {
		t.Fatalf("XDG_CONFIG_HOME=%q", got["XDG_CONFIG_HOME"])
	}
	if got["XDG_DATA_HOME"] != filepath.Join(cliDataRoot, "data") {
		t.Fatalf("XDG_DATA_HOME=%q", got["XDG_DATA_HOME"])
	}
	if got["XDG_CACHE_HOME"] != filepath.Join(cliDataRoot, "cache") {
		t.Fatalf("XDG_CACHE_HOME=%q", got["XDG_CACHE_HOME"])
	}
	if got["FOO"] != "BAR" {
		t.Fatalf("FOO inheritance lost: %q", got["FOO"])
	}
	if _, ok := got["NPM_CONFIG_PREFIX"]; ok {
		t.Fatalf("NPM_CONFIG_PREFIX must be stripped in isolated mode")
	}
	if !strings.HasPrefix(got["PATH"], filepath.Join(dataDir, "plugins", "cli", "lark-cli", "node_modules", ".bin")) {
		t.Fatalf("PATH prefix not applied: %q", got["PATH"])
	}
}

// TestBuildEnvSharedKeepsHome verifies shared mode preserves the user's real HOME and XDG_* values.
func TestBuildEnvSharedKeepsHome(t *testing.T) {
	env := BuildEnv(EnvParams{
		Isolation:  IsolationShared,
		CLIDataDir: "/ignored",
		PluginBin:  "/plugin/bin",
		ParentEnv:  []string{"HOME=/Users/me", "PATH=/usr/bin", "XDG_CONFIG_HOME=/userxdg"},
	})

	got := envMap(env)
	if got["HOME"] != "/Users/me" {
		t.Fatalf("HOME should be preserved, got %q", got["HOME"])
	}
	if got["XDG_CONFIG_HOME"] != "/userxdg" {
		t.Fatalf("XDG_CONFIG_HOME should be preserved, got %q", got["XDG_CONFIG_HOME"])
	}
	if !strings.HasPrefix(got["PATH"], "/plugin/bin") {
		t.Fatalf("PATH prefix not applied: %q", got["PATH"])
	}
}

// TestBuildEnvPrependsRuntimeBinBeforePluginBin verifies npm/npx resolve from the bundled runtime.
func TestBuildEnvPrependsRuntimeBinBeforePluginBin(t *testing.T) {
	env := BuildEnv(EnvParams{
		Isolation:  IsolationShared,
		CLIDataDir: "/ignored",
		RuntimeBin: "/runtime/bin",
		PluginBin:  "/plugin/bin",
		ParentEnv:  []string{"PATH=/usr/bin"},
	})

	got := envMap(env)
	wantPrefix := "/runtime/bin" + string(filepath.ListSeparator) + "/plugin/bin"
	if !strings.HasPrefix(got["PATH"], wantPrefix) {
		t.Fatalf("PATH should prefer runtime then plugin bin, got %q", got["PATH"])
	}
}

// envMap parses an exec.Cmd-style env slice ("KEY=VAL") into a map for assertion.
func envMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, e := range env {
		idx := strings.IndexByte(e, '=')
		if idx <= 0 {
			continue
		}
		out[e[:idx]] = e[idx+1:]
	}
	return out
}
