package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/reatang/fire-poker/backend/models/data_models"
	"github.com/reatang/fire-poker/backend/pkg/agent/toolkit"
	"github.com/reatang/fire-poker/backend/pkg/agent/tools"
	"github.com/reatang/fire-poker/backend/pkg/configfile"
	"github.com/reatang/fire-poker/backend/pkg/dir"
	"github.com/reatang/fire-poker/backend/pkg/id/event_id"
	"github.com/reatang/fire-poker/backend/pkg/id/prompt_id"
	pkgMCP "github.com/reatang/fire-poker/backend/pkg/mcp"
	"github.com/reatang/fire-poker/backend/pkg/prompt"
	pkgProvider "github.com/reatang/fire-poker/backend/pkg/provider"
	pkgterminal "github.com/reatang/fire-poker/backend/pkg/terminal"
	"github.com/reatang/fire-poker/backend/storage"
	"github.com/wailsapp/wails/v3/pkg/application"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/session/inmemory"
	toolpkg "trpc.group/trpc-go/trpc-agent-go/tool"
)

const defaultMainAgentPrompt = prompt.DefaultMainAgentPrompt
const defaultGenChatTitlePrompt = `Generate a concise title under 20 characters for this conversation. Return only the title.`

// Manager manages runners, prompts, and tool registration for the main agent.
type Manager struct {
	mu                      sync.RWMutex
	istorage                *storage.Storage
	wailsApp                *application.App
	toolRegistry            toolkit.Manager
	toolDependenciesVersion uint64
	streamManager           *StreamManager
	runners                 *runnerCache
	mcpManager              *pkgMCP.Manager
	skillProvider           tools.SkillProvider
	attention               tools.AttentionRequester
	skillCreator            tools.SkillCreator
	cliInstaller            tools.CliInstaller
	cliInstallProgress      tools.CliInstallProgressReporter
	cliManifestGenerator    tools.CliManifestGenerator
	cliCommandRunner        tools.CliCommandRunner
	terminalRunner          *pkgterminal.Manager
	toolConfirmDecider      ToolConfirmDecider
	memoryEncodeSem         chan struct{}
	sessionService          *inmemory.SessionService
}

type toolResultEmitter struct {
	app *application.App
}

func (e toolResultEmitter) EmitToolResult(sessionID uint, toolName, result string) {
	if e.app == nil {
		return
	}
	e.app.Event.Emit(event_id.AgentStreamToolResult, map[string]any{
		"sessionId": sessionID,
		"toolName":  toolName,
		"result":    result,
	})
}

// NewManager creates a new manager and registers the default main-agent prompt/tools.
func NewManager(istorage *storage.Storage) *Manager {
	prompt.Register(prompt_id.MainAgent, defaultMainAgentPrompt)
	prompt.Register(prompt_id.GenChatTitle, defaultGenChatTitlePrompt)

	m := &Manager{
		istorage:           istorage,
		toolRegistry:       toolkit.NewRegistry(),
		streamManager:      NewStreamManager(),
		runners:            newRunnerCache(maxCachedRunners),
		mcpManager:         pkgMCP.NewManager(mustDataDir()),
		toolConfirmDecider: defaultToolConfirmDecider,
		memoryEncodeSem:    make(chan struct{}, 2),
		sessionService:     inmemory.NewSessionService(),
	}
	if err := m.registerDefaultTools(); err != nil {
		panic(err)
	}
	return m
}

// SetApp stores the Wails app reference during service startup.
func (m *Manager) SetApp(app *application.App) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.wailsApp = app
	m.toolDependenciesVersion++
	m.runners.clear()
}

// ToolRegistry exposes the catalog and registration contract without its concrete implementation.
func (m *Manager) ToolRegistry() toolkit.Manager {
	return m.toolRegistry
}

// Streams exposes the active stream manager.
func (m *Manager) Streams() *StreamManager {
	return m.streamManager
}

// Storage exposes the backing storage.
func (m *Manager) Storage() *storage.Storage {
	return m.istorage
}

// App returns the Wails application handle.
func (m *Manager) App() *application.App {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.wailsApp
}

// SetSkillProvider stores the skill provider used by the Skill meta-tool.
// It must be called before any chat messages are processed.
func (m *Manager) SetSkillProvider(provider tools.SkillProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.skillProvider = provider
	m.toolDependenciesVersion++
	m.runners.clear()
}

// SkillProvider returns the currently configured skill provider, or nil.
func (m *Manager) SkillProvider() tools.SkillProvider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.skillProvider
}

// SetAttentionRequester stores the notification-backed attention requester.
func (m *Manager) SetAttentionRequester(requester tools.AttentionRequester) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.attention = requester
	m.toolDependenciesVersion++
	m.runners.clear()
}

// SetSkillCreator stores the persistence layer used by ProposeSkill.
func (m *Manager) SetSkillCreator(creator tools.SkillCreator) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.skillCreator = creator
	m.toolDependenciesVersion++
	m.runners.clear()
}

// SetCliInstaller stores the CLI installer used by the InstallCli tool.
func (m *Manager) SetCliInstaller(i tools.CliInstaller) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cliInstaller = i
	m.toolDependenciesVersion++
	m.runners.clear()
}

// SetCliInstallProgressReporter stores the progress reporter used by task install flows.
func (m *Manager) SetCliInstallProgressReporter(r tools.CliInstallProgressReporter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cliInstallProgress = r
	m.toolDependenciesVersion++
	m.runners.clear()
}

// SetCliManifestGenerator stores the manifest generator used by the GenerateCliManifest tool.
func (m *Manager) SetCliManifestGenerator(g tools.CliManifestGenerator) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cliManifestGenerator = g
	m.toolDependenciesVersion++
	m.runners.clear()
}

// SetCliCommandRunner stores the arbitrary CLI command runner used by the RunCliCommand tool.
func (m *Manager) SetCliCommandRunner(r tools.CliCommandRunner) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cliCommandRunner = r
	m.toolDependenciesVersion++
	m.runners.clear()
}

// SetTerminalRunner supplies the shared terminal service used by shell tools.
func (m *Manager) SetTerminalRunner(r *pkgterminal.Manager) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.terminalRunner = r
	m.toolDependenciesVersion++
	m.runners.clear()
}

// SetToolConfirmDecider overrides the runtime approval decider used before tool execution.
func (m *Manager) SetToolConfirmDecider(decider ToolConfirmDecider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.toolConfirmDecider = decider
}

// buildAgentTools delegates scoped construction to the implementation-independent manager.
func (m *Manager) buildAgentTools(enabledUserTools []string, sessionKind string, sessionID uint) ([]toolpkg.Tool, error) {
	return m.toolRegistry.BuildTools(toolkit.Scope{EnabledTools: enabledUserTools, SessionKind: sessionKind, SessionID: sessionID})
}

// inferOpenAIVariant maps the persisted provider type to the matching OpenAI variant.
func inferOpenAIVariant(providerType pkgProvider.Type) openai.Variant {
	switch providerType {
	case pkgProvider.Aliyun:
		return openai.VariantQwen
	case pkgProvider.Deepseek:
		return openai.VariantDeepSeek
	default:
		return openai.VariantOpenAI
	}
}

// GetOrCreateRunner caches session-scoped tool assemblies by model, credentials, catalog and service versions.
func (m *Manager) GetOrCreateRunner(
	cfg ModelConfig,
	enabledUserTools []string,
	sessionKind string,
	sessionID uint,
) (runner.Runner, error) {
	key := m.runnerKey(cfg, enabledUserTools, sessionKind, sessionID)
	if existing, ok := m.runners.get(key); ok {
		return existing, nil
	}

	instruction, err := prompt.Load(prompt_id.MainAgent)
	if err != nil {
		instruction = defaultMainAgentPrompt
	}

	variant := inferOpenAIVariant(cfg.ProviderType)

	mdl := openai.New(
		cfg.ModelName,
		openai.WithBaseURL(cfg.BaseURL),
		openai.WithAPIKey(cfg.APIKey),
		openai.WithVariant(variant),
	)

	agentTools, err := m.buildAgentTools(enabledUserTools, sessionKind, sessionID)
	if err != nil {
		return nil, err
	}
	toolSets, err := m.toolRegistry.BuildToolSets(toolkit.Scope{EnabledTools: enabledUserTools, SessionKind: sessionKind, SessionID: sessionID})
	if err != nil {
		return nil, err
	}
	ag := llmagent.New(
		"main",
		llmagent.WithModel(mdl),
		llmagent.WithInstruction(instruction),
		llmagent.WithGenerationConfig(model.GenerationConfig{
			Stream: true,
		}),
		llmagent.WithTools(agentTools),
		llmagent.WithToolSets(toolSets),
		llmagent.WithRefreshToolSetsOnRun(true),
		llmagent.WithToolCallbacks(m.newToolCallbacks()),
	)

	created := runner.NewRunner(
		"fire-poker",
		ag,
		runner.WithSessionService(m.sessionService),
	)

	return m.runners.add(key, created), nil
}

// runnerKey identifies a runner by provider, credentials, model, tool selection, session and dependency versions.
// The credential fingerprint makes an edited API key or base URL miss the cache instead of reusing a stale client.
func (m *Manager) runnerKey(cfg ModelConfig, enabledUserTools []string, sessionKind string, sessionID uint) string {
	names := slices.Clone(enabledUserTools)
	slices.Sort(names)
	selection, _ := json.Marshal(names)
	m.mu.RLock()
	servicesVersion := m.toolDependenciesVersion
	m.mu.RUnlock()
	return fmt.Sprintf("provider:%d|cred:%s|model:%s|tools:%s|kind:%s|session:%d|catalog:%d|services:%d",
		cfg.ProviderID, cfg.credentialFingerprint(), cfg.ModelName, selection, sessionKind, sessionID,
		m.toolRegistry.Version(), servicesVersion)
}

func loadAgentConfig() (*data_models.Config, error) {
	dataDir, err := dir.GetDataDir()
	if err != nil {
		return nil, err
	}
	bytes, err := configfile.ReadFile(filepath.Join(dataDir, dir.ConfigFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return &data_models.Config{
				Extensions: []data_models.ExtensionItem{},
				Memory:     data_models.MemoryConfig{Enabled: true},
			}, nil
		}
		return nil, err
	}

	config := &data_models.Config{}
	if err := json.Unmarshal(bytes, config); err != nil {
		return nil, err
	}
	if !strings.Contains(string(bytes), `"memory"`) {
		config.Memory.Enabled = true
	}
	if config.Extensions == nil {
		config.Extensions = []data_models.ExtensionItem{}
	}
	return config, nil
}

func mustDataDir() string {
	dataDir, err := dir.GetDataDir()
	if err != nil {
		return dir.HomeDir()
	}
	return dataDir
}

// NewChatHandler creates a chat handler bound to the manager.
func (m *Manager) NewChatHandler() *ChatHandler {
	return NewChatHandler(m)
}
