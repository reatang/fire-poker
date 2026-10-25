package plugin

import (
	"context"

	"github.com/reatang/fire-poker/backend/models/data_models"
	pkgcli "github.com/reatang/fire-poker/backend/pkg/cli"
	"github.com/reatang/fire-poker/backend/pkg/configfile"
	"github.com/reatang/fire-poker/backend/pkg/dir"
)

func mustDataDir() string {
	dataDir, err := dir.GetDataDir()
	if err != nil {
		return dir.HomeDir()
	}
	return dataDir
}

// loadConfig reads the shared config, creating first-run defaults so extensions can be saved into it.
func (p *Plugin) loadConfig() (*data_models.Config, error) {
	dataDir, err := dir.GetDataDir()
	if err != nil {
		return nil, err
	}
	return configfile.LoadOrInit(dataDir, &data_models.Config{
		Locale:     "zh-CN",
		Language:   "zh-CN",
		FontSize:   "md",
		DataDir:    dataDir,
		LogLevel:   "info",
		Extensions: []data_models.ExtensionItem{},
	})
}

func (p *Plugin) saveConfig(config *data_models.Config) error {
	return configfile.SaveFields(config.DataDir, config, "extensions")
}

func findExtension(items []data_models.ExtensionItem, id string) (data_models.ExtensionItem, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return data_models.ExtensionItem{}, false
}

func removeExtension(items []data_models.ExtensionItem, id string) []data_models.ExtensionItem {
	result := make([]data_models.ExtensionItem, 0, len(items))
	for _, item := range items {
		if item.ID != id {
			result = append(result, item)
		}
	}
	return result
}

// syncExtensionRuntime resolves runtime status and tool list for a single extension item.
// For cli plugins it projects tools from manifest.json; for mcp plugins it invokes the MCP manager to discover tools.
func (p *Plugin) syncExtensionRuntime(ctx context.Context, item data_models.ExtensionItem) data_models.ExtensionItem {
	if item.Kind == "cli" {
		manifest, err := pkgcli.LoadManifest(item.ConfigFilePath)
		if err != nil || manifest.Executable == "" {
			item.RuntimeStatus = "error"
			item.RuntimeMessage = "manifest missing; reinstall"
			item.Tools = []data_models.ExtensionTool{}
			return item
		}
		item.Tools = projectCliTools(item, manifest)
		if item.Enabled {
			item.RuntimeStatus = "ready"
			item.RuntimeMessage = ""
		} else {
			item.RuntimeStatus = "idle"
			item.RuntimeMessage = ""
		}
		return item
	}
	if item.Kind != "mcp" {
		item.RuntimeStatus = "ready"
		item.RuntimeMessage = ""
		return item
	}
	if !item.Enabled {
		item.Tools = []data_models.ExtensionTool{}
		item.RuntimeStatus = "idle"
		item.RuntimeMessage = ""
		return item
	}

	tools, err := p.mcpManager.DiscoverTools(ctx, item)
	if err != nil {
		item.Tools = []data_models.ExtensionTool{}
		item.RuntimeStatus = "error"
		item.RuntimeMessage = err.Error()
		return item
	}

	item.Tools = tools
	item.RuntimeStatus = "ready"
	item.RuntimeMessage = ""
	return item
}

func projectCliTools(item data_models.ExtensionItem, manifest pkgcli.Manifest) []data_models.ExtensionTool {
	result := make([]data_models.ExtensionTool, 0, len(manifest.Tools))
	for _, current := range manifest.Tools {
		result = append(result, data_models.ExtensionTool{
			ToolID:          pkgcli.CliToolID(item.Name, current.Name),
			ServerID:        item.ID,
			Name:            current.Name,
			Description:     current.Description,
			Enabled:         current.Enabled,
			RequiresConfirm: current.RequiresConfirm,
		})
	}
	return result
}
