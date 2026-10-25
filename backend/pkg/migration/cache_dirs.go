package migration

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/reatang/fire-poker/backend/pkg/dir"
)

// runtimeStateFileName mirrors the Node runtime state file whose absolute paths must follow a move.
const runtimeStateFileName = "state.json"

// runtimeStatePathKeys lists the state.json fields that hold absolute paths under the runtime root.
var runtimeStatePathKeys = []string{"install_dir", "node_path", "npm_path"}

// MigrateCacheDirs moves re-creatable data from its legacy location into {dataDir}/cache.
// Each move is independent and idempotent; failures are reported as warnings so startup continues
// on the legacy location instead of losing data.
func MigrateCacheDirs(dataDir string) []string {
	var warnings []string

	legacyRuntime := dir.LegacyNodeRuntimeRoot(dataDir)
	runtimeRoot := dir.NodeRuntimeRoot(dataDir)
	moved, warn, err := moveLegacy(legacyRuntime, runtimeRoot)
	warnings = appendWarning(warnings, warn, err)
	if moved {
		if err := rewriteRuntimeStatePaths(runtimeRoot, legacyRuntime); err != nil {
			warnings = append(warnings, "rewrite node runtime state: "+err.Error())
		}
		// Drop the now-empty legacy runtime/ parent; a non-empty one is left untouched.
		_ = os.Remove(filepath.Dir(legacyRuntime))
	}

	_, warn, err = moveLegacy(dir.LegacyWebViewDataRoot(dataDir), dir.WebViewDataRoot(dataDir))
	warnings = appendWarning(warnings, warn, err)
	return warnings
}

// WebViewDataDir returns the WebView2 directory to use, staying on the legacy one if migration could not move it.
func WebViewDataDir(dataDir string) string {
	legacy := dir.LegacyWebViewDataRoot(dataDir)
	if info, err := os.Stat(legacy); err == nil && info.IsDir() {
		return legacy
	}
	return dir.WebViewDataRoot(dataDir)
}

// appendWarning records a move warning or error message when present.
func appendWarning(warnings []string, warn string, err error) []string {
	if warn != "" {
		warnings = append(warnings, warn)
	}
	if err != nil {
		warnings = append(warnings, err.Error())
	}
	return warnings
}

// rewriteRuntimeStatePaths points absolute paths in state.json at the new runtime root.
// Unknown fields are preserved because the file is edited as a generic JSON object.
func rewriteRuntimeStatePaths(runtimeRoot string, legacyRoot string) error {
	statePath := filepath.Join(runtimeRoot, runtimeStateFileName)
	bytes, err := os.ReadFile(statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	var state map[string]any
	if err := json.Unmarshal(bytes, &state); err != nil {
		return err
	}

	changed := false
	for _, key := range runtimeStatePathKeys {
		value, ok := state[key].(string)
		if !ok {
			continue
		}
		if rewritten, ok := rebasePath(value, legacyRoot, runtimeRoot); ok {
			state[key] = rewritten
			changed = true
		}
	}
	if !changed {
		return nil
	}

	updated, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := statePath + ".tmp"
	if err := os.WriteFile(tmpPath, updated, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, statePath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

// rebasePath swaps the oldRoot prefix of value for newRoot, matching whole path segments only.
func rebasePath(value string, oldRoot string, newRoot string) (string, bool) {
	if value == oldRoot {
		return newRoot, true
	}
	prefix := oldRoot + string(filepath.Separator)
	if strings.HasPrefix(value, prefix) {
		return filepath.Join(newRoot, value[len(prefix):]), true
	}
	return value, false
}
