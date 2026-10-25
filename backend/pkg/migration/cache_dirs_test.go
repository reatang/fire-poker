package migration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/reatang/fire-poker/backend/pkg/dir"
)

// TestMigrateCacheDirsMovesRuntimeAndRewritesState covers the upgrade path from runtime/node to cache/runtime/node.
func TestMigrateCacheDirsMovesRuntimeAndRewritesState(t *testing.T) {
	dataDir := t.TempDir()
	legacy := dir.LegacyNodeRuntimeRoot(dataDir)
	nodePath := filepath.Join(legacy, "node-v22", "bin", "node")
	if err := os.MkdirAll(filepath.Dir(nodePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nodePath, []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	state := map[string]any{
		"state":       "ready",
		"install_dir": filepath.Join(legacy, "node-v22"),
		"node_path":   nodePath,
		"npm_path":    filepath.Join(legacy, "node-v22", "bin", "npm"),
		"error_msg":   "",
		"future_key":  "kept",
	}
	writeJSON(t, filepath.Join(legacy, "state.json"), state)

	if warnings := MigrateCacheDirs(dataDir); len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	root := dir.NodeRuntimeRoot(dataDir)
	if _, err := os.Stat(filepath.Join(root, "node-v22", "bin", "node")); err != nil {
		t.Fatalf("node binary not moved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "runtime")); !os.IsNotExist(err) {
		t.Fatalf("empty legacy runtime dir should be removed, stat err %v", err)
	}

	var got map[string]any
	readJSON(t, filepath.Join(root, "state.json"), &got)
	want := map[string]string{
		"install_dir": filepath.Join(root, "node-v22"),
		"node_path":   filepath.Join(root, "node-v22", "bin", "node"),
		"npm_path":    filepath.Join(root, "node-v22", "bin", "npm"),
		"state":       "ready",
		"future_key":  "kept",
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("%s = %v, want %q", key, got[key], value)
		}
	}

	// A second run must be a no-op.
	if warnings := MigrateCacheDirs(dataDir); len(warnings) != 0 {
		t.Fatalf("second run should be silent, got %v", warnings)
	}
}

// TestMigrateCacheDirsKeepsBothWhenTargetExists never merges or overwrites an existing cache directory.
func TestMigrateCacheDirsKeepsBothWhenTargetExists(t *testing.T) {
	dataDir := t.TempDir()
	for _, path := range []string{dir.LegacyNodeRuntimeRoot(dataDir), dir.NodeRuntimeRoot(dataDir)} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	warnings := MigrateCacheDirs(dataDir)
	if len(warnings) != 1 {
		t.Fatalf("expected one warning about both dirs existing, got %v", warnings)
	}
	if _, err := os.Stat(dir.LegacyNodeRuntimeRoot(dataDir)); err != nil {
		t.Fatalf("legacy runtime must be left in place: %v", err)
	}
}

// TestWebViewDataDirFallsBackToLegacy keeps WebView storage reachable when the legacy directory could not be moved.
func TestWebViewDataDirFallsBackToLegacy(t *testing.T) {
	dataDir := t.TempDir()
	if got := WebViewDataDir(dataDir); got != dir.WebViewDataRoot(dataDir) {
		t.Fatalf("fresh install should use cache path, got %q", got)
	}

	legacy := dir.LegacyWebViewDataRoot(dataDir)
	if err := os.MkdirAll(legacy, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := WebViewDataDir(dataDir); got != legacy {
		t.Fatalf("unmoved legacy dir should win, got %q", got)
	}

	if warnings := MigrateCacheDirs(dataDir); len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}
	if got := WebViewDataDir(dataDir); got != dir.WebViewDataRoot(dataDir) {
		t.Fatalf("migrated install should use cache path, got %q", got)
	}
}

// writeJSON writes value to path as JSON for test fixtures.
func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	bytes, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, bytes, 0o644); err != nil {
		t.Fatal(err)
	}
}

// readJSON decodes the JSON file at path into target.
func readJSON(t *testing.T, path string, target any) {
	t.Helper()
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(bytes, target); err != nil {
		t.Fatal(err)
	}
}
