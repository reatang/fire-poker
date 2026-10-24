package agent

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/reatang/fire-poker/backend/models/data_models"
	"github.com/reatang/fire-poker/backend/pkg/agent/toolkit"
)

// TestExtensionProvidersSkipUnselected avoids touching unavailable runtimes for unrelated tools.
func TestExtensionProvidersSkipUnselected(t *testing.T) {
	writeAgentConfig(t, data_models.Config{Extensions: []data_models.ExtensionItem{
		{ID: "cli:broken", Kind: "cli", Enabled: true, ConfigFilePath: "missing.json"},
		{ID: "mcp:broken", Kind: "mcp", Enabled: true, ConfigFilePath: "missing.json"},
	}})
	m := NewManager(nil)
	sets, err := m.ToolRegistry().BuildToolSets(toolkit.Scope{EnabledTools: []string{"code_exec"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 0 {
		t.Fatal("unselected extensions were loaded")
	}
}

// TestExtensionProviderFailureReachesCaller prevents silent removal of a selected capability.
func TestExtensionProviderFailureReachesCaller(t *testing.T) {
	writeAgentConfig(t, data_models.Config{Extensions: []data_models.ExtensionItem{
		{ID: "mcp:broken", Kind: "mcp", Enabled: true, ConfigFilePath: filepath.Join(t.TempDir(), "missing.json")},
	}})
	m := NewManager(nil)
	_, err := m.ToolRegistry().BuildToolSets(toolkit.Scope{EnabledTools: []string{"mcp:broken"}})
	if err == nil || !strings.Contains(err.Error(), "mcp:broken") {
		t.Fatalf("missing extension identity: %v", err)
	}
}

// TestSelectedExtensionsHonorsIndividualSelectionAndDisabledGroups preserves extension selection semantics.
func TestSelectedExtensionsHonorsIndividualSelectionAndDisabledGroups(t *testing.T) {
	writeAgentConfig(t, data_models.Config{Extensions: []data_models.ExtensionItem{
		{ID: "mcp:one", Kind: "mcp", Enabled: true, Tools: []data_models.ExtensionTool{{ToolID: "one_read"}}},
		{ID: "mcp:disabled", Kind: "mcp", Enabled: false},
	}})
	items, err := selectedExtensions("mcp", []string{"one_read", "mcp:disabled"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "mcp:one" {
		t.Fatalf("selection: %+v", items)
	}
}
