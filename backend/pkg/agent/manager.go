package agent

import (
	"encoding/json"
	"fmt"
	"slices"
	"sync"

	"github.com/reatang/fire-poker/backend/pkg/agent/toolkit"
	"github.com/reatang/fire-poker/backend/pkg/agent/tools"
	"github.com/reatang/fire-poker/backend/pkg/dir"
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

// ManagerDeps groups the optional application services tools depend on. They are fixed at
// construction; a nil field leaves the corresponding tool unavailable.
type ManagerDeps struct {
	SkillProvider        tools.SkillProvider
	AttentionRequester   tools.AttentionRequester
	SkillCreator         tools.SkillCreator
	CliInstaller         tools.CliInstaller
	CliInstallProgress   tools.CliInstallProgressReporter
	CliManifestGenerator tools.CliManifestGenerator
	CliCommandRunner     tools.CliCommandRunner
	TerminalRunner       *pkgterminal.Manager
}

// Manager manages runners, prompts, and tool registration for the main agent.
type Manager struct {
	mu       sync.RWMutex
	istorage *storage.Storage
	// deps is immutable after construction, so tool factories read it without locking.
	deps     ManagerDeps
	wailsApp *application.App
	// appVersion changes when the Wails app handle is replaced, invalidating runners whose tools captured it.
	appVersion         uint64
	toolRegistry       toolkit.Manager
	streamManager      *StreamManager
	runners            *runnerCache
	mcpManager         *pkgMCP.Manager
	toolConfirmDecider ToolConfirmDecider
	memoryEncodeSem    chan struct{}
	sessionService     *inmemory.SessionService
}

type toolResultEmitter struct {
	app *application.App
}

// EmitToolResult forwards progress that long-running tools report before they return.
func (e toolResultEmitter) EmitToolResult(sessionID uint, toolName, result string) {
	emitStreamToolResult(e.app, StreamToolResultEvent{SessionID: sessionID, ToolName: toolName, Result: result})
}

// NewManager creates a manager without optional tool services; tests and minimal setups use it.
func NewManager(istorage *storage.Storage) *Manager {
	return NewManagerWithDeps(istorage, ManagerDeps{})
}

// NewManagerWithDeps creates a manager wired to application services and registers the default prompts/tools.
func NewManagerWithDeps(istorage *storage.Storage, deps ManagerDeps) *Manager {
	prompt.Register(prompt_id.MainAgent, defaultMainAgentPrompt)
	prompt.Register(prompt_id.GenChatTitle, defaultGenChatTitlePrompt)

	m := &Manager{
		istorage:           istorage,
		deps:               deps,
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
	m.appVersion++
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

// SetToolConfirmDecider overrides the runtime approval decider used before tool execution.
func (m *Manager) SetToolConfirmDecider(decider ToolConfirmDecider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.toolConfirmDecider = decider
}

// buildAgentTools delegates scoped construction to the implementation-independent manager.
func (m *Manager) buildAgentTools(enabledUserTools []string, sessionKind string, sessionID uint) ([]toolpkg.Tool, error) {
	return m.toolRegistry.BuildTools(toolkit.Scope{EnabledTools: enabledUserTools, SessionKind: sessionKind, SessionID: sessionID, WorkDir: m.SessionWorkDir(sessionID)})
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
	toolSets, err := m.toolRegistry.BuildToolSets(toolkit.Scope{EnabledTools: enabledUserTools, SessionKind: sessionKind, SessionID: sessionID, WorkDir: m.SessionWorkDir(sessionID)})
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
	servicesVersion := m.appVersion
	m.mu.RUnlock()
	return fmt.Sprintf("provider:%d|cred:%s|model:%s|tools:%s|kind:%s|session:%d|catalog:%d|services:%d",
		cfg.ProviderID, cfg.credentialFingerprint(), cfg.ModelName, selection, sessionKind, sessionID,
		m.toolRegistry.Version(), servicesVersion)
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
