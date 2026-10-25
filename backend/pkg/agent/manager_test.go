package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reatang/fire-poker/backend/models/data_models"
	"github.com/reatang/fire-poker/backend/pkg/agent/tools"
	"github.com/reatang/fire-poker/backend/pkg/dir"
	"github.com/reatang/fire-poker/backend/pkg/id/prompt_id"
	"github.com/reatang/fire-poker/backend/pkg/prompt"
	pkgProvider "github.com/reatang/fire-poker/backend/pkg/provider"
	"github.com/reatang/fire-poker/backend/storage"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"trpc.group/trpc-go/trpc-agent-go/runner"
)

// createTestProvider stores an enabled provider so turns can resolve credentials by ID.
func createTestProvider(t *testing.T, manager *Manager, baseURL string, providerType pkgProvider.Type) uint {
	t.Helper()
	provider, err := manager.Storage().CreateProvider(data_models.Provider{
		ProviderName: "test",
		ProviderType: providerType,
		BaseUrl:      baseURL,
		ApiKey:       "test-key",
		Enable:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return provider.ID
}

// injectSessionRunner stores a provider and registers a fake under the actual session-scoped
// cache identity, returning the provider ID the turn must send.
func injectSessionRunner(t *testing.T, manager *Manager, sessionID uint, baseURL, modelName string, providerType pkgProvider.Type, fake runner.Runner) uint {
	t.Helper()
	providerID := createTestProvider(t, manager, baseURL, providerType)
	cfg, err := manager.ResolveModelConfig(providerID, modelName)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := manager.SessionToolIDs(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	session, err := manager.Storage().GetSession(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	manager.runners.add(manager.runnerKey(cfg, ids, session.Kind, sessionID), fake)
	return providerID
}

type fakeCliInstallProgressReporter struct{}

func (f *fakeCliInstallProgressReporter) ReportCliInstallProgress(_ context.Context, _ uint, _ tools.CliInstallProgressItem) error {
	return nil
}

func newTestStorage(t *testing.T) *storage.Storage {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	stor, err := storage.NewStorageFromDB(db)
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	return stor
}

func writeAgentConfig(t *testing.T, cfg data_models.Config) {
	t.Helper()

	dataDir := t.TempDir()
	t.Setenv("FIRE_POKER_DATA_DIR", dataDir)

	bytes, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, dir.ConfigFileName), bytes, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func writeMCPConfigFile(t *testing.T) string {
	t.Helper()

	rootDir := t.TempDir()
	configPath := filepath.Join(rootDir, "mcp.json")
	if err := os.WriteFile(configPath, []byte(`{
  "transport": "stdio",
  "command": "node",
  "args": ["server.js"],
  "description": "Browser tools"
}`), 0o644); err != nil {
		t.Fatalf("write mcp config: %v", err)
	}
	return configPath
}

func TestNewManagerRegistersPromptAndBuiltInTools(t *testing.T) {
	prompt.Register(prompt_id.MainAgent, "")
	prompt.Register(prompt_id.GenChatTitle, "")

	manager := NewManager(nil)

	loadedPrompt, err := prompt.Load(prompt_id.MainAgent)
	if err != nil {
		t.Fatalf("load prompt: %v", err)
	}
	if loadedPrompt == "" {
		t.Fatal("expected main agent prompt to be registered")
	}

	titlePrompt, err := prompt.Load(prompt_id.GenChatTitle)
	if err != nil {
		t.Fatalf("load title prompt: %v", err)
	}
	if titlePrompt == "" {
		t.Fatal("expected title prompt to be registered")
	}

	if manager.ToolRegistry() == nil {
		t.Fatal("expected tool registry to be initialized")
	}
	if manager.Streams() == nil {
		t.Fatal("expected stream manager to be initialized")
	}

	builtinTools := manager.ToolRegistry().BuiltinTools()
	if len(builtinTools) != 19 {
		t.Fatalf("expected 19 builtin tools, got %d", len(builtinTools))
	}

	userTools := manager.ToolRegistry().UserTools()
	if len(userTools) != 1 {
		t.Fatalf("expected 1 user tool, got %d", len(userTools))
	}
}

func TestBuildAgentToolsIncludesBuiltinAndEnabledUserTools(t *testing.T) {
	manager := NewManager(nil)

	agentTools, err := manager.buildAgentTools([]string{"code_exec"}, "user", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(agentTools) != 1 || agentTools[0].Declaration().Name != "code_exec" {
		t.Fatalf("expected only the selected code_exec tool, got %v", agentTools)
	}
}

func TestBuildAgentToolsIncludesWebToolsByDefault(t *testing.T) {
	manager := NewManager(nil)

	agentTools, err := manager.buildAgentTools(nil, "user", 1)
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool, len(agentTools))
	for _, current := range agentTools {
		names[current.Declaration().Name] = true
	}

	for _, want := range []string{"web_fetch", tools.WebSearchToolName} {
		if !names[want] {
			t.Fatalf("expected builtin web tool %q to be available by default, got %v", want, names)
		}
	}
}

func TestBuildAgentToolsIncludesTaskStateToolsForTaskSessions(t *testing.T) {
	manager := NewManager(nil)
	manager.SetCliInstallProgressReporter(&fakeCliInstallProgressReporter{})

	agentTools, err := manager.buildAgentTools(nil, "task", 1)
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool, len(agentTools))
	for _, current := range agentTools {
		names[current.Declaration().Name] = true
	}
	if !names[tools.SaveTaskStateToolName] || !names[tools.LoadTaskStateToolName] {
		t.Fatalf("expected task state tools in task session, got %v", names)
	}
	if !names[tools.ReportCliInstallProgressToolName] {
		t.Fatalf("expected cli install progress tool in task session, got %v", names)
	}
}

func TestBuildAgentToolsIncludesOrdinaryUserTools(t *testing.T) {
	manager := NewManager(nil)

	agentTools, err := manager.buildAgentTools([]string{"web_search"}, "user", 1)
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool, len(agentTools))
	for _, current := range agentTools {
		names[current.Declaration().Name] = true
	}

	for _, want := range []string{tools.WebSearchToolName} {
		if !names[want] {
			t.Fatalf("expected ordinary user tool %q to be available, got %v", want, names)
		}
	}
}

func TestBuildMCPToolSetsReturnsNoneWhenNoExtensionToolIsSelected(t *testing.T) {
	configPath := writeMCPConfigFile(t)
	writeAgentConfig(t, data_models.Config{
		Extensions: []data_models.ExtensionItem{
			{
				ID:             "mcp:browser:1.0.0",
				Name:           "browser",
				Description:    "Browser tools",
				Kind:           "mcp",
				Enabled:        true,
				ConfigFilePath: configPath,
			},
		},
	})

	manager := NewManager(nil)

	toolSets, err := manager.buildMCPToolSets([]string{})
	if err != nil {
		t.Fatal(err)
	}
	if len(toolSets) != 0 {
		t.Fatalf("expected 0 mcp tool sets when no extension tool is selected, got %d", len(toolSets))
	}
}

func TestGetOrCreateRunnerSeparatesRunnersByToolSelection(t *testing.T) {
	manager := NewManager(nil)
	cfg := ModelConfig{ProviderID: 1, BaseURL: "https://api.example.com/v1", APIKey: "key-1", ModelName: "gpt-test", ProviderType: pkgProvider.OpenAiCompatibility}

	first, err := manager.GetOrCreateRunner(cfg, []string{"code_exec"}, "user", 1)
	if err != nil {
		t.Fatalf("first runner: %v", err)
	}

	second, err := manager.GetOrCreateRunner(cfg, []string{"web_fetch"}, "user", 1)
	if err != nil {
		t.Fatalf("second runner: %v", err)
	}

	if first == second {
		t.Fatal("expected different runners when enabled tools differ")
	}
}

// TestGetOrCreateRunnerRebuildsAfterCredentialChange proves an edited API key or base URL
// is not served by a runner built with the old credentials.
func TestGetOrCreateRunnerRebuildsAfterCredentialChange(t *testing.T) {
	manager := NewManager(nil)
	cfg := ModelConfig{ProviderID: 1, BaseURL: "https://api.example.com/v1", APIKey: "old-key", ModelName: "gpt-test", ProviderType: pkgProvider.OpenAiCompatibility}

	original, err := manager.GetOrCreateRunner(cfg, nil, "user", 1)
	if err != nil {
		t.Fatalf("original runner: %v", err)
	}
	same, err := manager.GetOrCreateRunner(cfg, nil, "user", 1)
	if err != nil {
		t.Fatalf("same runner: %v", err)
	}
	if original != same {
		t.Fatal("expected unchanged config to reuse the cached runner")
	}

	rotated := cfg
	rotated.APIKey = "new-key"
	afterKey, err := manager.GetOrCreateRunner(rotated, nil, "user", 1)
	if err != nil {
		t.Fatalf("rotated-key runner: %v", err)
	}
	if afterKey == original {
		t.Fatal("expected a new runner after the API key changed")
	}

	moved := rotated
	moved.BaseURL = "https://proxy.example.com/v1"
	afterURL, err := manager.GetOrCreateRunner(moved, nil, "user", 1)
	if err != nil {
		t.Fatalf("moved-url runner: %v", err)
	}
	if afterURL == afterKey {
		t.Fatal("expected a new runner after the base URL changed")
	}
}

// TestRunnerKeyOmitsAPIKey keeps raw credentials out of cache keys that may surface in logs or dumps.
func TestRunnerKeyOmitsAPIKey(t *testing.T) {
	manager := NewManager(nil)
	key := manager.runnerKey(ModelConfig{ProviderID: 1, BaseURL: "https://api.example.com", APIKey: "sk-very-secret", ModelName: "m"}, nil, "user", 1)
	if strings.Contains(key, "sk-very-secret") {
		t.Fatalf("runner key leaked the API key: %s", key)
	}
}

func TestResolveModelConfigLoadsStoredCredentials(t *testing.T) {
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	manager := NewManager(newTestStorage(t))
	providerID := createTestProvider(t, manager, "https://api.example.com/v1", pkgProvider.Deepseek)

	cfg, err := manager.ResolveModelConfig(providerID, "  deepseek-chat  ")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	want := ModelConfig{ProviderID: providerID, BaseURL: "https://api.example.com/v1", APIKey: "test-key", ModelName: "deepseek-chat", ProviderType: pkgProvider.Deepseek}
	if cfg != want {
		t.Fatalf("resolved %+v, want %+v", cfg, want)
	}
}

func TestResolveModelConfigRejectsInvalidSelections(t *testing.T) {
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	manager := NewManager(newTestStorage(t))
	enabledID := createTestProvider(t, manager, "https://api.example.com/v1", pkgProvider.OpenAiCompatibility)
	disabledID := createTestProvider(t, manager, "https://disabled.example.com/v1", pkgProvider.OpenAiCompatibility)
	disabled, err := manager.Storage().GetProvider(disabledID)
	if err != nil {
		t.Fatal(err)
	}
	// Enable has a GORM default of true, so a disabled row must be written by update rather than create.
	disabled.Enable = false
	if err := manager.Storage().UpdateProvider(*disabled); err != nil {
		t.Fatal(err)
	}
	deletedID := createTestProvider(t, manager, "https://deleted.example.com/v1", pkgProvider.OpenAiCompatibility)
	if err := manager.Storage().DeleteProviderCascade(deletedID); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name       string
		providerID uint
		modelName  string
		want       error
	}{
		{name: "missing provider id", providerID: 0, modelName: "m", want: ErrProviderRequired},
		{name: "blank model", providerID: enabledID, modelName: "  ", want: ErrModelRequired},
		{name: "unknown provider", providerID: 9999, modelName: "m", want: ErrProviderNotFound},
		{name: "deleted provider", providerID: deletedID, modelName: "m", want: ErrProviderNotFound},
		{name: "disabled provider", providerID: disabledID, modelName: "m", want: ErrProviderDisabled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := manager.ResolveModelConfig(tc.providerID, tc.modelName); !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
}

// TestSendMessageWithUnknownProviderLeavesNoUserMessage proves credential resolution happens before any write.
func TestSendMessageWithUnknownProviderLeavesNoUserMessage(t *testing.T) {
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	manager := NewManager(newTestStorage(t))
	session, err := manager.Storage().CreateSession(data_models.Session{Title: "no provider"})
	if err != nil {
		t.Fatal(err)
	}

	err = NewChatHandler(manager).SendMessage(context.Background(), SendMessageParams{SessionID: session.ID, Content: "hello", ProviderID: 9999, ModelName: "m"})
	if !errors.Is(err, ErrProviderNotFound) {
		t.Fatalf("expected provider-not-found error, got %v", err)
	}
	count, err := manager.Storage().CountMessagesForSession(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected no persisted messages, got %d", count)
	}
	if manager.Streams().IsActive(session.ID) {
		t.Fatal("expected the stream reservation to be released")
	}
}

func TestInferOpenAIVariantFromProviderType(t *testing.T) {
	tests := []struct {
		name         string
		providerType pkgProvider.Type
		expected     string
	}{
		{
			name:         "aliyun maps to qwen variant",
			providerType: pkgProvider.Aliyun,
			expected:     "qwen",
		},
		{
			name:         "deepseek maps to deepseek variant",
			providerType: pkgProvider.Deepseek,
			expected:     "deepseek",
		},
		{
			name:         "openai compatibility defaults to openai variant",
			providerType: pkgProvider.OpenAiCompatibility,
			expected:     "openai",
		},
		{
			name:         "unknown provider falls back to openai variant",
			providerType: pkgProvider.Type("custom"),
			expected:     "openai",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferOpenAIVariant(tt.providerType)
			if string(got) != tt.expected {
				t.Fatalf("inferOpenAIVariant(%q) = %q, want %q", tt.providerType, got, tt.expected)
			}
		})
	}
}
