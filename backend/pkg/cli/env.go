package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// EnvParams gathers the inputs needed to assemble an isolated or shared environment.
type EnvParams struct {
	Isolation  IsolationMode
	CLIDataDir string   // absolute path to {data_dir}/plugins/cli_data/{name}
	RuntimeBin string   // absolute path to bundled runtime bin dir; npm/npx must resolve here first
	PluginBin  string   // absolute path to {install_dir}/node_modules/.bin
	ParentEnv  []string // typically os.Environ()
}

// strippedEnvKeys are removed from the inherited environment to prevent the host's npm config
// or NODE_PATH from leaking into the isolated subprocess.
var strippedEnvKeys = []string{"NPM_CONFIG_PREFIX", "NODE_PATH"}

// overriddenKeysIsolated are env vars that BuildEnv unconditionally replaces in isolated mode.
var overriddenKeysIsolated = []string{
	"HOME",
	"USERPROFILE",
	"XDG_CONFIG_HOME",
	"XDG_DATA_HOME",
	"XDG_CACHE_HOME",
	"APPDATA",
	"LOCALAPPDATA",
	"TEMP",
	"TMP",
	"TMPDIR",
	"NPM_CONFIG_CACHE",
	"NPM_CONFIG_LOGS_DIR",
	"NPM_CONFIG_USERCONFIG",
	"NPM_CONFIG_GLOBALCONFIG",
}

// PrepareEnv creates private directories before starting a plugin. Shared mode
// deliberately retains the user's directories; this is environment isolation,
// not a filesystem sandbox.
func PrepareEnv(p EnvParams) ([]string, error) {
	if p.Isolation != IsolationShared {
		if p.CLIDataDir == "" || !filepath.IsAbs(p.CLIDataDir) {
			return nil, fmt.Errorf("cli: isolated environment requires an absolute data directory")
		}
		for _, subdir := range []string{"home", "config", "data", "cache", "appdata/roaming", "appdata/local", "tmp", "cache/npm", "cache/npm/_logs"} {
			if err := os.MkdirAll(filepath.Join(p.CLIDataDir, filepath.FromSlash(subdir)), 0o700); err != nil {
				return nil, fmt.Errorf("cli: prepare private directory: %w", err)
			}
		}
	}
	return BuildEnv(p), nil
}

// BuildEnv returns an exec.Cmd-style environment slice for a CLI subprocess.
// In isolated mode, HOME / USERPROFILE / XDG_* are pointed at subdirs of CLIDataDir;
// in shared mode the parent's values for those keys are preserved.
// In both modes RuntimeBin and PluginBin are prepended to PATH and strippedEnvKeys are removed.
func BuildEnv(p EnvParams) []string {
	out := make([]string, 0, len(p.ParentEnv)+8)

	skipKeys := map[string]struct{}{}
	for _, k := range strippedEnvKeys {
		skipKeys[k] = struct{}{}
	}
	if p.Isolation != IsolationShared {
		for _, k := range overriddenKeysIsolated {
			skipKeys[k] = struct{}{}
		}
	}

	for _, entry := range p.ParentEnv {
		idx := strings.IndexByte(entry, '=')
		if idx <= 0 {
			continue
		}
		key := entry[:idx]
		// Windows environment names and npm's config environment names are
		// case-insensitive (npm also accepts npm_config_cache on Unix).
		lookupKey := key
		if runtime.GOOS == "windows" || strings.HasPrefix(strings.ToUpper(key), "NPM_CONFIG_") {
			lookupKey = strings.ToUpper(key)
		}
		if lookupKey == "PATH" {
			out = append(out, "PATH="+prependPath(pathPrefix(p.RuntimeBin, p.PluginBin), entry[idx+1:]))
			continue
		}
		if _, skip := skipKeys[lookupKey]; skip {
			continue
		}
		out = append(out, entry)
	}

	if hasNoPath(p.ParentEnv) {
		out = append(out, "PATH="+pathPrefix(p.RuntimeBin, p.PluginBin))
	}

	if p.Isolation != IsolationShared {
		out = append(out,
			"HOME="+filepath.Join(p.CLIDataDir, "home"),
			"USERPROFILE="+filepath.Join(p.CLIDataDir, "home"),
			"XDG_CONFIG_HOME="+filepath.Join(p.CLIDataDir, "config"),
			"XDG_DATA_HOME="+filepath.Join(p.CLIDataDir, "data"),
			"XDG_CACHE_HOME="+filepath.Join(p.CLIDataDir, "cache"),
			"APPDATA="+filepath.Join(p.CLIDataDir, "appdata", "roaming"),
			"LOCALAPPDATA="+filepath.Join(p.CLIDataDir, "appdata", "local"),
			"TEMP="+filepath.Join(p.CLIDataDir, "tmp"),
			"TMP="+filepath.Join(p.CLIDataDir, "tmp"),
			"TMPDIR="+filepath.Join(p.CLIDataDir, "tmp"),
			"NPM_CONFIG_CACHE="+filepath.Join(p.CLIDataDir, "cache", "npm"),
			"NPM_CONFIG_LOGS_DIR="+filepath.Join(p.CLIDataDir, "cache", "npm", "_logs"),
			"NPM_CONFIG_USERCONFIG="+filepath.Join(p.CLIDataDir, "config", "npmrc"),
			"NPM_CONFIG_GLOBALCONFIG="+filepath.Join(p.CLIDataDir, "config", "npmrc-global"),
		)
	}
	return out
}

func pathPrefix(parts ...string) string {
	nonEmpty := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			nonEmpty = append(nonEmpty, part)
		}
	}
	return strings.Join(nonEmpty, pathListSeparator())
}

// prependPath returns "<bin>{sep}<existing>", skipping empty bin.
func prependPath(bin string, existing string) string {
	if bin == "" {
		return existing
	}
	return bin + pathListSeparator() + existing
}

func pathListSeparator() string {
	if runtime.GOOS == "windows" {
		return ";"
	}
	return ":"
}

// hasNoPath reports whether the parent env lacks a PATH entry.
func hasNoPath(env []string) bool {
	for _, e := range env {
		if strings.HasPrefix(e, "PATH=") || (runtime.GOOS == "windows" && strings.HasPrefix(strings.ToUpper(e), "PATH=")) {
			return false
		}
	}
	return true
}
