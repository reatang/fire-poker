package agent

import (
	"fmt"

	"github.com/reatang/fire-poker/backend/models/data_models"
	"github.com/reatang/fire-poker/backend/pkg/agent/toolkit"
	pkgcli "github.com/reatang/fire-poker/backend/pkg/cli"
	"github.com/reatang/fire-poker/backend/pkg/configfile"
	pkgRuntimeState "github.com/reatang/fire-poker/backend/pkg/runtime_state"
	agenttool "trpc.group/trpc-go/trpc-agent-go/tool"
)

// registerExtensionProviders composes dynamic extension sources through the registrar contract.
func (m *Manager) registerExtensionProviders(registrar toolkit.Registrar) error {
	if err := registrar.RegisterProvider("mcp", func(scope toolkit.Scope) ([]agenttool.ToolSet, error) {
		return m.buildMCPToolSets(scope.EnabledTools)
	}); err != nil {
		return err
	}
	return registrar.RegisterProvider("cli", func(scope toolkit.Scope) ([]agenttool.ToolSet, error) {
		return m.buildCliToolSets(scope.EnabledTools)
	})
}

// selectedExtensions resolves only enabled extensions selected as a group or by individual tool ID.
func selectedExtensions(kind string, enabledToolIDs []string) ([]data_models.ExtensionItem, error) {
	if len(enabledToolIDs) == 0 {
		return nil, nil
	}
	config, err := configfile.LoadCurrent()
	if err != nil {
		return nil, fmt.Errorf("load extension configuration: %w", err)
	}
	selected := make(map[string]bool, len(enabledToolIDs))
	for _, id := range enabledToolIDs {
		selected[id] = true
	}
	var result []data_models.ExtensionItem
	for _, item := range config.Extensions {
		if item.Kind != kind || !item.Enabled {
			continue
		}
		included := selected[item.ID]
		for _, current := range item.Tools {
			if selected[current.ToolID] {
				included = true
				break
			}
		}
		if included {
			result = append(result, item)
		}
	}
	return result, nil
}

// buildMCPToolSets resolves selected MCP extensions and reports configuration failures.
func (m *Manager) buildMCPToolSets(enabledUserTools []string) ([]agenttool.ToolSet, error) {
	items, err := selectedExtensions("mcp", enabledUserTools)
	if err != nil {
		return nil, err
	}
	var result []agenttool.ToolSet
	for _, item := range items {
		set, err := m.mcpManager.BuildToolSet(item, enabledUserTools)
		if err != nil {
			return nil, fmt.Errorf("MCP extension %q: %w", item.ID, err)
		}
		result = append(result, set)
	}
	return result, nil
}

// buildCliToolSets resolves selected CLI extensions without requiring a runtime for unrelated tools.
func (m *Manager) buildCliToolSets(enabledUserTools []string) ([]agenttool.ToolSet, error) {
	items, err := selectedExtensions("cli", enabledUserTools)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	state, err := pkgRuntimeState.LoadPersistedState()
	if err != nil {
		return nil, fmt.Errorf("load CLI runtime: %w", err)
	}
	if state.State != "ready" || state.NodePath == "" || state.NpmPath == "" {
		return nil, fmt.Errorf("CLI runtime is not ready for selected extensions")
	}
	mgr := pkgcli.NewManager(mustDataDir(), state.NodePath, state.NpmPath)
	var result []agenttool.ToolSet
	for _, item := range items {
		manifest, err := pkgcli.LoadManifest(item.ConfigFilePath)
		if err != nil {
			return nil, fmt.Errorf("CLI extension %q manifest: %w", item.ID, err)
		}
		if manifest.Executable == "" {
			return nil, fmt.Errorf("CLI extension %q has no executable", item.ID)
		}
		set, err := pkgcli.BuildToolSet(mgr, item, manifest, enabledUserTools)
		if err != nil {
			return nil, fmt.Errorf("CLI extension %q: %w", item.ID, err)
		}
		result = append(result, set)
	}
	return result, nil
}
