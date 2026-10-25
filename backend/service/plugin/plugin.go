package plugin

import (
	"context"
	"errors"
	"os"
	"sync"

	"github.com/reatang/fire-poker/backend/models/data_models"
	agenttools "github.com/reatang/fire-poker/backend/pkg/agent/tools"
	pkgcli "github.com/reatang/fire-poker/backend/pkg/cli"
	"github.com/reatang/fire-poker/backend/pkg/configfile"
	"github.com/reatang/fire-poker/backend/pkg/ierror"
	pkgmcp "github.com/reatang/fire-poker/backend/pkg/mcp"
	pkgProvider "github.com/reatang/fire-poker/backend/pkg/provider"
	pkgRuntimeState "github.com/reatang/fire-poker/backend/pkg/runtime_state"
	"github.com/reatang/fire-poker/backend/service/plugin/plugin_dto"
	"github.com/reatang/fire-poker/backend/storage"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Plugin manages imported MCP tools and future plugin entries.
type Plugin struct {
	mcpManager              *pkgmcp.Manager
	cliManager              *pkgcli.Manager
	probeCliHelp            func(ctx context.Context, executable string, env []string) (string, error)
	generateManifest        func(ctx context.Context, params pkgcli.GenerateParams) (pkgcli.Manifest, error)
	resolveDefaultChatModel func() (defaultChatModel, error)
	loadPersistedRuntime    func() (pkgRuntimeState.StateSnapshot, error)
	wailsApp                *application.App
	loginMu                 sync.Mutex //nolint:unused // only the !windows CLI login implementation locks it
	loginFields                        //nolint:unused // platform-specific: loginSessions + startCliLogin (no-op stub on Windows)
}

// NewPlugin creates a plugin service without database access; manifest generation needs NewPluginWithStorage.
func NewPlugin() *Plugin {
	return NewPluginWithStorage(nil)
}

// NewPluginWithStorage creates a plugin service that reads the default chat model from the shared storage.
func NewPluginWithStorage(stor *storage.Storage) *Plugin {
	p := &Plugin{
		mcpManager:       pkgmcp.NewManager(mustDataDir()),
		probeCliHelp:     pkgcli.ProbeHelp,
		generateManifest: pkgcli.Generate,
		resolveDefaultChatModel: func() (defaultChatModel, error) {
			return loadDefaultChatModel(stor)
		},
		loadPersistedRuntime: pkgRuntimeState.LoadPersistedState,
	}
	initLoginFields(p)
	return p
}

// resolveCliManager constructs (lazily) the CLI manager by reading node/npm paths from runtime state.json.
// Returns an error if the runtime is not yet downloaded / ready.
func (p *Plugin) resolveCliManager() (*pkgcli.Manager, error) {
	if p.cliManager != nil {
		return p.cliManager, nil
	}
	state, err := p.loadPersistedRuntime()
	if err != nil {
		return nil, err
	}
	if state.State != "ready" || state.NodePath == "" || state.NpmPath == "" {
		return nil, ierror.Error(ierror.ErrCliRuntimeUnavailable, errors.New("plugin runtime not ready"))
	}
	p.cliManager = pkgcli.NewManager(mustDataDir(), state.NodePath, state.NpmPath)
	return p.cliManager, nil
}

type defaultChatModel struct {
	BaseURL      string
	APIKey       string
	ModelName    string
	ProviderType pkgProvider.Type
}

// errPluginStorageUnavailable reports a plugin service built without the shared database.
var errPluginStorageUnavailable = errors.New("plugin service has no storage")

// loadDefaultChatModel resolves the persisted default provider plus its selected default model.
// It reuses the application's storage; opening a fresh database per call leaked connections.
func loadDefaultChatModel(stor *storage.Storage) (defaultChatModel, error) {
	if stor == nil {
		return defaultChatModel{}, errPluginStorageUnavailable
	}
	providers, err := stor.ListProviders()
	if err != nil {
		return defaultChatModel{}, err
	}
	cfg, err := configfile.LoadCurrent()
	if err != nil {
		return defaultChatModel{}, err
	}
	defaultProviderID := cfg.DefaultProviderID
	if defaultProviderID == 0 {
		return defaultChatModel{}, os.ErrNotExist
	}
	for _, provider := range providers {
		if provider.ID != defaultProviderID {
			continue
		}
		defaultModel, err := stor.GetDefaultModel(provider.ID)
		if err != nil {
			return defaultChatModel{}, err
		}
		if defaultModel == nil {
			return defaultChatModel{}, os.ErrNotExist
		}
		models, err := stor.ListModelsForProvider(provider.ID)
		if err != nil {
			return defaultChatModel{}, err
		}
		for _, current := range models {
			if current.ID == defaultModel.ModelId {
				return defaultChatModel{
					BaseURL:      provider.BaseUrl,
					APIKey:       provider.ApiKey,
					ModelName:    current.Model,
					ProviderType: provider.ProviderType,
				}, nil
			}
		}
		return defaultChatModel{}, os.ErrNotExist
	}
	return defaultChatModel{}, os.ErrNotExist
}

// ListExtensions returns all imported plugin and MCP entries from persisted config.
func (p *Plugin) ListExtensions(ctx context.Context, input plugin_dto.ListExtensionsInput) (*plugin_dto.ListExtensionsOutput, error) {
	config, err := p.loadConfig()
	if err != nil {
		return nil, ierror.Error(ierror.ErrSettingsLoadConfig, err)
	}
	changed := false
	items := make([]data_models.ExtensionItem, 0, len(config.Extensions))
	for _, item := range config.Extensions {
		next := item
		if (item.Kind == "mcp" || item.Kind == "cli") && (len(item.Tools) == 0 || item.RuntimeStatus == "") {
			next = p.syncExtensionRuntime(ctx, item)
			changed = changed || next.RuntimeStatus != item.RuntimeStatus || next.RuntimeMessage != item.RuntimeMessage || len(next.Tools) != len(item.Tools)
		}
		items = append(items, next)
	}
	if changed {
		config.Extensions = items
		if err := p.saveConfig(config); err != nil {
			return nil, ierror.Error(ierror.ErrSettingsSaveConfig, err)
		}
	}
	return &plugin_dto.ListExtensionsOutput{Extensions: items}, nil
}

// ImportExtension imports a local extension directory into the app data dir and persists it to config.
func (p *Plugin) ImportExtension(ctx context.Context, input plugin_dto.ImportExtensionInput) (*plugin_dto.ImportExtensionOutput, error) {
	if input.Kind != "mcp" && input.Kind != "plugin" {
		return nil, ierror.Error(ierror.ErrSettingsSaveConfig, os.ErrInvalid)
	}

	config, err := p.loadConfig()
	if err != nil {
		return nil, ierror.Error(ierror.ErrSettingsLoadConfig, err)
	}

	var item data_models.ExtensionItem
	if input.Kind == "plugin" {
		item, err = p.mcpManager.ImportPlugin(ctx, pkgmcp.ImportInput{
			SourceDir: input.Path,
			Enabled:   true,
		})
	} else {
		item, err = p.mcpManager.ImportMCP(ctx, pkgmcp.ImportInput{
			SourceDir: input.Path,
			Enabled:   true,
		})
	}
	if err != nil {
		return nil, ierror.Error(ierror.ErrSettingsSaveConfig, err)
	}

	item = p.syncExtensionRuntime(ctx, item)

	config.Extensions = append(removeExtension(config.Extensions, item.ID), item)
	if err := p.saveConfig(config); err != nil {
		return nil, ierror.Error(ierror.ErrSettingsSaveConfig, err)
	}

	return &plugin_dto.ImportExtensionOutput{Extension: item}, nil
}

// GetExtensionDetail returns one extension plus its editable config file text.
func (p *Plugin) GetExtensionDetail(ctx context.Context, input plugin_dto.GetExtensionDetailInput) (*plugin_dto.GetExtensionDetailOutput, error) {
	_ = ctx
	config, err := p.loadConfig()
	if err != nil {
		return nil, ierror.Error(ierror.ErrSettingsLoadConfig, err)
	}
	item, ok := findExtension(config.Extensions, input.ID)
	if !ok {
		return nil, ierror.Error(ierror.ErrSettingsReadConfig, os.ErrNotExist)
	}
	if item.ConfigFilePath == "" {
		return &plugin_dto.GetExtensionDetailOutput{
			Extension:  item,
			ConfigText: "",
		}, nil
	}
	bytes, err := os.ReadFile(item.ConfigFilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &plugin_dto.GetExtensionDetailOutput{Extension: item, ConfigText: ""}, nil
		}
		return nil, ierror.Error(ierror.ErrSettingsReadConfig, err)
	}
	return &plugin_dto.GetExtensionDetailOutput{
		Extension:  item,
		ConfigText: string(bytes),
	}, nil
}

// ToggleExtension updates the enabled state and refreshes discovered tools when enabling.
func (p *Plugin) ToggleExtension(ctx context.Context, input plugin_dto.ToggleExtensionInput) (*plugin_dto.ToggleExtensionOutput, error) {
	config, err := p.loadConfig()
	if err != nil {
		return nil, ierror.Error(ierror.ErrSettingsLoadConfig, err)
	}
	item, ok := findExtension(config.Extensions, input.ID)
	if !ok {
		return nil, ierror.Error(ierror.ErrSettingsReadConfig, os.ErrNotExist)
	}
	item.Enabled = input.Enabled
	item = p.syncExtensionRuntime(ctx, item)
	config.Extensions = append(removeExtension(config.Extensions, item.ID), item)
	if err := p.saveConfig(config); err != nil {
		return nil, ierror.Error(ierror.ErrSettingsSaveConfig, err)
	}
	return &plugin_dto.ToggleExtensionOutput{Extension: item}, nil
}

// ReloadExtension re-discovers tools from the current on-disk config for one extension.
func (p *Plugin) ReloadExtension(ctx context.Context, input plugin_dto.ReloadExtensionInput) (*plugin_dto.ReloadExtensionOutput, error) {
	config, err := p.loadConfig()
	if err != nil {
		return nil, ierror.Error(ierror.ErrSettingsLoadConfig, err)
	}
	item, ok := findExtension(config.Extensions, input.ID)
	if !ok {
		return nil, ierror.Error(ierror.ErrSettingsReadConfig, os.ErrNotExist)
	}
	item = p.syncExtensionRuntime(ctx, item)
	config.Extensions = append(removeExtension(config.Extensions, item.ID), item)
	if err := p.saveConfig(config); err != nil {
		return nil, ierror.Error(ierror.ErrSettingsSaveConfig, err)
	}
	return &plugin_dto.ReloadExtensionOutput{Extension: item}, nil
}

// DeleteExtension removes one imported extension from config and deletes its install directory.
// For CLI plugins this preserves plugins/cli_data/<name>/ (login state, manifest) by design;
// use ResetCliData to clear that separately.
func (p *Plugin) DeleteExtension(ctx context.Context, input plugin_dto.DeleteExtensionInput) (*plugin_dto.DeleteExtensionOutput, error) {
	_ = ctx
	config, err := p.loadConfig()
	if err != nil {
		return nil, ierror.Error(ierror.ErrSettingsLoadConfig, err)
	}
	item, ok := findExtension(config.Extensions, input.ID)
	if !ok {
		return nil, ierror.Error(ierror.ErrSettingsReadConfig, os.ErrNotExist)
	}
	if err := os.RemoveAll(item.RootDir); err != nil {
		return nil, ierror.Error(ierror.ErrSettingsSaveConfig, err)
	}
	config.Extensions = removeExtension(config.Extensions, item.ID)
	if err := p.saveConfig(config); err != nil {
		return nil, ierror.Error(ierror.ErrSettingsSaveConfig, err)
	}
	return &plugin_dto.DeleteExtensionOutput{}, nil
}

// SaveExtensionConfig writes the edited config file and refreshes discovered tools.
func (p *Plugin) SaveExtensionConfig(ctx context.Context, input plugin_dto.SaveExtensionConfigInput) (*plugin_dto.SaveExtensionConfigOutput, error) {
	config, err := p.loadConfig()
	if err != nil {
		return nil, ierror.Error(ierror.ErrSettingsLoadConfig, err)
	}
	item, ok := findExtension(config.Extensions, input.ID)
	if !ok {
		return nil, ierror.Error(ierror.ErrSettingsReadConfig, os.ErrNotExist)
	}
	if item.Kind == "cli" {
		out, err := p.UpdateCliManifest(ctx, plugin_dto.UpdateCliManifestInput{ID: input.ID, ManifestText: input.ConfigText})
		if err != nil {
			return nil, err
		}
		return &plugin_dto.SaveExtensionConfigOutput{Extension: out.Extension}, nil
	}
	if item.ConfigFilePath == "" {
		return &plugin_dto.SaveExtensionConfigOutput{Extension: item}, nil
	}
	if err := os.WriteFile(item.ConfigFilePath, []byte(input.ConfigText), 0o644); err != nil {
		return nil, ierror.Error(ierror.ErrSettingsSaveConfig, err)
	}
	item = p.syncExtensionRuntime(ctx, item)
	config.Extensions = append(removeExtension(config.Extensions, item.ID), item)
	if err := p.saveConfig(config); err != nil {
		return nil, ierror.Error(ierror.ErrSettingsSaveConfig, err)
	}
	return &plugin_dto.SaveExtensionConfigOutput{Extension: item}, nil
}

// ListAvailableTools returns builtin tools plus all enabled extension tool groups currently discoverable from config.
func (p *Plugin) ListAvailableTools(ctx context.Context, input plugin_dto.ListAvailableToolsInput) (*plugin_dto.ListAvailableToolsOutput, error) {
	config, err := p.loadConfig()
	if err != nil {
		return nil, ierror.Error(ierror.ErrSettingsLoadConfig, err)
	}

	result := make([]plugin_dto.ToolItem, 0)
	for _, meta := range builtinToolMetas() {
		result = append(result, plugin_dto.ToolItem{
			ID:          meta.Name,
			Name:        meta.Name,
			Description: meta.Description,
			Category:    meta.Category,
		})
	}

	for _, item := range config.Extensions {
		if !item.Enabled || (item.Kind != "mcp" && item.Kind != "cli") {
			continue
		}
		resolved := item
		if len(resolved.Tools) == 0 {
			resolved = p.syncExtensionRuntime(ctx, item)
		}
		if len(resolved.Tools) == 0 {
			continue
		}
		result = append(result, plugin_dto.ToolItem{
			ID:          resolved.ID,
			Name:        resolved.Name,
			Description: resolved.Description,
			Category:    resolved.Kind,
		})
	}

	return &plugin_dto.ListAvailableToolsOutput{Tools: result}, nil
}

// builtinToolMetas shares the runtime catalog instead of maintaining a second tool list.
func builtinToolMetas() []agenttools.ToolMeta {
	return agenttools.ListedToolMetas()
}
