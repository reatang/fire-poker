package plugin

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/reatang/fire-poker/backend/models/data_models"
	agenttools "github.com/reatang/fire-poker/backend/pkg/agent/tools"
	"github.com/reatang/fire-poker/backend/pkg/configfile"
	"github.com/reatang/fire-poker/backend/service/plugin/plugin_dto"
	"github.com/reatang/fire-poker/backend/service/settings"
	"github.com/reatang/fire-poker/backend/service/settings/settings_dto"
	"github.com/reatang/fire-poker/backend/storage"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestDelayedPluginSavePreservesAppliedTools reproduces a plugin probe committing a stale config snapshot.
func TestDelayedPluginSavePreservesAppliedTools(t *testing.T) {
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	ctx := context.Background()
	s := settings.NewSettings(settings.Dependencies{})
	if _, err := s.SaveToolSettings(ctx, settings_dto.SaveToolSettingsInput{DefaultToolIDs: []string{"shell"}}); err != nil {
		t.Fatal(err)
	}
	p := NewPlugin()
	stale, err := p.loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveToolSettings(ctx, settings_dto.SaveToolSettingsInput{DefaultToolIDs: []string{}}); err != nil {
		t.Fatal(err)
	}
	stale.Extensions = []data_models.ExtensionItem{{ID: "mcp:test", RuntimeStatus: "ready"}}
	if err := p.saveConfig(stale); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.LoadBootstrap(ctx, settings_dto.LoadBootstrapInput{})
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Bootstrap.DefaultToolIDs == nil || len(loaded.Bootstrap.DefaultToolIDs) != 0 {
		t.Fatal("plugin refresh restored unchecked tools")
	}
}

func writeTestMCPBundleDir(t *testing.T) string {
	t.Helper()

	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "manifest.json"), []byte(`{
  "name": "filesystem",
  "version": "1.0.0",
  "description": "Filesystem tools",
  "author": "Fire Poker",
  "config_file": "mcp.json"
}`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "mcp.json"), []byte(`{
  "transport": "stdio",
  "command": "node",
  "args": ["server.js"],
  "description": "Filesystem tools"
}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return rootDir
}

func newTestPluginService(t *testing.T) *Plugin {
	t.Helper()

	tempDataDir := t.TempDir()
	t.Setenv("FIRE_POKER_DATA_DIR", tempDataDir)
	return NewPlugin()
}

func TestListExtensionsReturnsImportedItems(t *testing.T) {
	service := newTestPluginService(t)
	ctx := context.Background()

	_, err := service.ImportExtension(ctx, plugin_dto.ImportExtensionInput{
		Kind: "mcp",
		Path: writeTestMCPBundleDir(t),
	})
	if err != nil {
		t.Fatalf("import extension: %v", err)
	}

	out, err := service.ListExtensions(ctx, plugin_dto.ListExtensionsInput{})
	if err != nil {
		t.Fatalf("list extensions: %v", err)
	}
	if len(out.Extensions) != 1 {
		t.Fatalf("expected 1 extension, got %d", len(out.Extensions))
	}
	if out.Extensions[0].Name != "filesystem" {
		t.Fatalf("expected filesystem extension, got %q", out.Extensions[0].Name)
	}
}

func TestListExtensionsPopulatesRuntimeErrorWhenDiscoveryFails(t *testing.T) {
	service := newTestPluginService(t)
	ctx := context.Background()

	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "mcp.json"), []byte(`{
  "mcpServers": {
    "broken": {
      "command": "definitely-missing-command",
      "args": []
    }
  }
}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := service.ImportExtension(ctx, plugin_dto.ImportExtensionInput{
		Kind: "mcp",
		Path: rootDir,
	})
	if err != nil {
		t.Fatalf("import extension: %v", err)
	}

	out, err := service.ListExtensions(ctx, plugin_dto.ListExtensionsInput{})
	if err != nil {
		t.Fatalf("list extensions: %v", err)
	}
	if len(out.Extensions) != 1 {
		t.Fatalf("expected 1 extension, got %d", len(out.Extensions))
	}
	if out.Extensions[0].RuntimeStatus != "error" {
		t.Fatalf("expected runtime error status, got %q", out.Extensions[0].RuntimeStatus)
	}
	if out.Extensions[0].RuntimeMessage == "" {
		t.Fatalf("expected runtime error message to be populated")
	}
}

func TestListAvailableToolsAggregatesEnabledMCPByServer(t *testing.T) {
	service := newTestPluginService(t)
	ctx := context.Background()

	cfg, err := service.loadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.Extensions = []data_models.ExtensionItem{
		{
			ID:          "mcp:filesystem:1.0.0",
			Name:        "filesystem",
			Description: "Filesystem tools",
			Kind:        "mcp",
			Enabled:     true,
			Tools: []data_models.ExtensionTool{
				{ToolID: "mcp:filesystem:1.0.0_read_file", Name: "read_file", Description: "Read a file"},
				{ToolID: "mcp:filesystem:1.0.0_write_file", Name: "write_file", Description: "Write a file"},
			},
		},
	}
	if err := service.saveConfig(cfg); err != nil {
		t.Fatalf("save config: %v", err)
	}

	out, err := service.ListAvailableTools(ctx, plugin_dto.ListAvailableToolsInput{})
	if err != nil {
		t.Fatalf("list available tools: %v", err)
	}

	var mcpTools []plugin_dto.ToolItem
	for _, item := range out.Tools {
		if item.Category == "mcp" {
			mcpTools = append(mcpTools, item)
		}
	}

	if len(mcpTools) != 1 {
		t.Fatalf("expected 1 aggregated mcp entry, got %d", len(mcpTools))
	}
	if mcpTools[0].ID != "mcp:filesystem:1.0.0" {
		t.Fatalf("expected aggregated mcp id, got %q", mcpTools[0].ID)
	}
	if mcpTools[0].Name != "filesystem" {
		t.Fatalf("expected mcp name filesystem, got %q", mcpTools[0].Name)
	}
}

func TestListAvailableToolsOnlyShowsPracticalBuiltinsForOrdinaryUsers(t *testing.T) {
	service := newTestPluginService(t)

	out, err := service.ListAvailableTools(context.Background(), plugin_dto.ListAvailableToolsInput{})
	if err != nil {
		t.Fatalf("list available tools: %v", err)
	}

	got := map[string]bool{}
	for _, item := range out.Tools {
		got[item.ID] = true
	}

	for _, want := range []string{agenttools.ShellMeta().Name, "question", "web_fetch", "web_search", "Skill", "todo_write"} {
		if !got[want] {
			t.Fatalf("expected practical builtin %q to be visible, got %v", want, got)
		}
	}
	for _, hidden := range []string{"file_read", "file_write", "code_exec"} {
		if got[hidden] {
			t.Fatalf("expected developer builtin %q to stay hidden, got %v", hidden, got)
		}
	}
}

// TestLoadDefaultChatModelUsesSharedStorage resolves the default model through the injected database.
func TestLoadDefaultChatModelUsesSharedStorage(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("FIRE_POKER_DATA_DIR", dataDir)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	stor, err := storage.NewStorageFromDB(db)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := stor.CreateProvider(data_models.Provider{ProviderName: "p", ProviderType: "deepseek", BaseUrl: "https://api.example.com", ApiKey: "k", Enable: true})
	if err != nil {
		t.Fatal(err)
	}
	models := []data_models.Model{{ProviderId: provider.ID, Model: "other"}, {ProviderId: provider.ID, Model: "deepseek-chat"}}
	if err := stor.CreateModels(&models); err != nil {
		t.Fatal(err)
	}
	if err := stor.UpsertDefaultModel(provider.ID, models[1].ID); err != nil {
		t.Fatal(err)
	}
	if err := configfile.SaveFields(dataDir, &data_models.Config{DefaultProviderID: provider.ID}, "default_provider_id"); err != nil {
		t.Fatal(err)
	}

	got, err := NewPluginWithStorage(stor).resolveDefaultChatModel()
	if err != nil {
		t.Fatalf("resolve default chat model: %v", err)
	}
	want := defaultChatModel{BaseURL: "https://api.example.com", APIKey: "k", ModelName: "deepseek-chat", ProviderType: "deepseek"}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// TestLoadDefaultChatModelWithoutStorage fails cleanly instead of opening a private database.
func TestLoadDefaultChatModelWithoutStorage(t *testing.T) {
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	if _, err := NewPlugin().resolveDefaultChatModel(); !errors.Is(err, errPluginStorageUnavailable) {
		t.Fatalf("expected errPluginStorageUnavailable, got %v", err)
	}
}
