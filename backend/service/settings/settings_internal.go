package settings

import (
	"errors"
	"path/filepath"

	"github.com/reatang/fire-poker/backend/models/data_models"
	"github.com/reatang/fire-poker/backend/pkg/configfile"
	"github.com/reatang/fire-poker/backend/pkg/dir"
	"github.com/reatang/fire-poker/backend/pkg/global_hotkey"
	"github.com/reatang/fire-poker/backend/pkg/ierror"
	"github.com/reatang/fire-poker/backend/service/settings/settings_dto"
)

// loadConfig reads the current config file and returns default values when it does not exist yet.
func (s *Settings) loadConfig() (*data_models.Config, error) {
	dataDir, err := dir.GetDataDir()
	if err != nil {
		return nil, ierror.Error(ierror.ErrSettingsLoadConfig, err)
	}

	config, err := configfile.LoadOrInit(dataDir, defaultConfig(dataDir))
	if err != nil {
		if errors.Is(err, configfile.ErrInvalidConfig) {
			return nil, ierror.Error(ierror.ErrSettingsParseConfig, err)
		}
		return nil, ierror.Error(ierror.ErrSettingsReadConfig, err)
	}
	// configfile.Load already resolved DataDir and the memory default; fill the display defaults.
	if config.FontSize == "" {
		config.FontSize = "md"
	}
	if config.LogLevel == "" {
		config.LogLevel = "info"
	}
	if config.Locale == "" {
		config.Locale = "zh-CN"
	}
	if config.Language == "" {
		config.Language = "zh-CN"
	}
	if config.QuickChatShortcut == "" {
		config.QuickChatShortcut = global_hotkey.DefaultQuickChatShortcut()
	}
	return config, nil
}

// saveConfigToDir writes a config file into the provided directory.
func (s *Settings) saveConfigToDir(targetDir string, config *data_models.Config, fields ...string) error {
	return ierror.Error(ierror.ErrSettingsSaveConfig, configfile.SaveFields(targetDir, config, fields...))
}

// defaultConfig builds the baseline config values used for first-time startup.
func defaultConfig(dataDir string) *data_models.Config {
	return &data_models.Config{
		Locale:            "zh-CN",
		Language:          "zh-CN",
		FontSize:          "md",
		DataDir:           dataDir,
		LogLevel:          "info",
		QuickChatShortcut: global_hotkey.DefaultQuickChatShortcut(),
		Memory: data_models.MemoryConfig{
			Enabled: true,
		},
	}
}

// buildDataDirectoryItems lists app-owned locations in display order; macOS adds the WebKit stores the system keeps outside dataDir.
func buildDataDirectoryItems(dataDir string, goos string, homeDir string, tempDir string) []settings_dto.DataDirectoryItem {
	folder := func(id string, path string) settings_dto.DataDirectoryItem {
		return settings_dto.DataDirectoryItem{ID: id, Path: path, RevealDir: path}
	}
	file := func(id string, path string) settings_dto.DataDirectoryItem {
		return settings_dto.DataDirectoryItem{ID: id, Path: path, RevealDir: filepath.Dir(path), IsFile: true}
	}

	items := []settings_dto.DataDirectoryItem{
		folder("data_root", dataDir),
		file("config", filepath.Join(dataDir, dir.ConfigFileName)),
		file("database", filepath.Join(dataDir, dir.DataBaseFileName)),
		folder("sessions", dir.SessionsRoot(dataDir)),
		folder("skills", dir.SkillsRoot(dataDir)),
		folder("plugins", dir.ExtensionsRoot(dataDir)),
		folder("logs", dir.LogsRoot(dataDir)),
		folder("cache", dir.CacheRoot(dataDir)),
	}
	if goos == "darwin" && homeDir != "" {
		items = append(items,
			folder("webview_data", filepath.Join(homeDir, "Library", "WebKit", dir.BundleIdentifier)),
			folder("webview_cache", filepath.Join(homeDir, "Library", "Caches", dir.BundleIdentifier)),
		)
	}
	return append(items, folder("temp", tempDir))
}
