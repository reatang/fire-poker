package settings

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/reatang/fire-poker/backend/models/data_models"
	"github.com/reatang/fire-poker/backend/pkg/dir"
	"github.com/reatang/fire-poker/backend/pkg/global_hotkey"
	"github.com/reatang/fire-poker/backend/service/settings/settings_dto"
)

// newTestSettingsService creates a Settings instance for testing.
func newTestSettingsService(t *testing.T) *Settings {
	t.Helper()
	return &Settings{}
}

type fakeShortcutController struct {
	current string
	status  global_hotkey.ShortcutValidationStatus
	updated []string
}

func (f *fakeShortcutController) CurrentShortcut() string {
	if f.current == "" {
		return global_hotkey.DefaultQuickChatShortcut()
	}
	return f.current
}

func (f *fakeShortcutController) ValidateShortcut(shortcut string) global_hotkey.ShortcutValidationResult {
	normalized, err := global_hotkey.NormalizeQuickChatShortcut(shortcut)
	if err != nil {
		return global_hotkey.ShortcutValidationResult{Status: global_hotkey.ShortcutValidationInvalid, Message: err.Error()}
	}
	status := f.status
	if status == "" {
		status = global_hotkey.ShortcutValidationAvailable
	}
	return global_hotkey.ShortcutValidationResult{Shortcut: normalized, Status: status}
}

func (f *fakeShortcutController) UpdateShortcut(ctx context.Context, shortcut string) error {
	if f.status == global_hotkey.ShortcutValidationConflict {
		return errors.New("shortcut already taken")
	}
	f.updated = append(f.updated, shortcut)
	f.current = shortcut
	return nil
}

// TestSaveLocaleSettingsPersistsLocaleAndLanguage verifies locale settings are written back to config.json.
func TestSaveLocaleSettingsPersistsLocaleAndLanguage(t *testing.T) {
	tempDataDir := t.TempDir()
	t.Setenv("FIRE_POKER_DATA_DIR", tempDataDir)

	svc := newTestSettingsService(t)
	if err := os.WriteFile(filepath.Join(tempDataDir, "config.json"), []byte(`{"locale":"zh-CN","language":"zh-CN","font_size":"md","data_dir":"`+filepath.ToSlash(tempDataDir)+`","log_level":"info"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := svc.SaveLocaleSettings(context.Background(), settings_dto.SaveLocaleSettingsInput{
		Locale:   "en-US",
		Language: "en",
	})
	if err != nil {
		t.Fatal(err)
	}

	bytes, err := os.ReadFile(filepath.Join(tempDataDir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}

	var cfg data_models.Config
	if err := json.Unmarshal(bytes, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Locale != "en-US" {
		t.Fatalf("expected locale to be persisted, got %q", cfg.Locale)
	}
	if cfg.Language != "en" {
		t.Fatalf("expected language to be persisted, got %q", cfg.Language)
	}
}

// TestSaveDisplaySettingsPersistsFontSize verifies display settings are written back to config.json.
func TestSaveDisplaySettingsPersistsFontSize(t *testing.T) {
	tempDataDir := t.TempDir()
	t.Setenv("FIRE_POKER_DATA_DIR", tempDataDir)

	svc := newTestSettingsService(t)
	if err := os.WriteFile(filepath.Join(tempDataDir, "config.json"), []byte(`{"locale":"zh-CN","language":"zh-CN","font_size":"md","data_dir":"`+filepath.ToSlash(tempDataDir)+`","log_level":"info"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := svc.SaveDisplaySettings(context.Background(), settings_dto.SaveDisplaySettingsInput{FontSize: "lg"})
	if err != nil {
		t.Fatal(err)
	}

	bytes, err := os.ReadFile(filepath.Join(tempDataDir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}

	var cfg data_models.Config
	if err := json.Unmarshal(bytes, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.FontSize != "lg" {
		t.Fatalf("expected font_size to be persisted, got %q", cfg.FontSize)
	}
}

func TestLoadBootstrapIncludesDefaultQuickChatShortcut(t *testing.T) {
	tempDataDir := t.TempDir()
	t.Setenv("FIRE_POKER_DATA_DIR", tempDataDir)

	svc := newTestSettingsService(t)
	if err := os.WriteFile(filepath.Join(tempDataDir, "config.json"), []byte(`{"locale":"zh-CN","language":"zh-CN","font_size":"md","data_dir":"`+filepath.ToSlash(tempDataDir)+`","log_level":"info"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := svc.LoadBootstrap(context.Background(), settings_dto.LoadBootstrapInput{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Bootstrap.QuickChatShortcut != global_hotkey.DefaultQuickChatShortcut() {
		t.Fatalf("quick_chat_shortcut = %q, want default", result.Bootstrap.QuickChatShortcut)
	}
}

func TestValidateShortcutSettingsReturnsConflictStatus(t *testing.T) {
	tempDataDir := t.TempDir()
	t.Setenv("FIRE_POKER_DATA_DIR", tempDataDir)

	controller := &fakeShortcutController{current: "CmdOrCtrl+Shift+Space", status: global_hotkey.ShortcutValidationConflict}
	svc := NewSettings(Dependencies{QuickChatShortcuts: controller})

	result, err := svc.ValidateShortcutSettings(context.Background(), settings_dto.ValidateShortcutSettingsInput{QuickChatShortcut: "CmdOrCtrl+Shift+K"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != string(global_hotkey.ShortcutValidationConflict) {
		t.Fatalf("status = %q, want conflict", result.Status)
	}
}

func TestSaveShortcutSettingsPersistsAndAppliesQuickChatShortcut(t *testing.T) {
	tempDataDir := t.TempDir()
	t.Setenv("FIRE_POKER_DATA_DIR", tempDataDir)

	if err := os.WriteFile(filepath.Join(tempDataDir, "config.json"), []byte(`{"locale":"zh-CN","language":"zh-CN","font_size":"md","data_dir":"`+filepath.ToSlash(tempDataDir)+`","log_level":"info","quick_chat_shortcut":"CmdOrCtrl+Shift+Space"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	controller := &fakeShortcutController{current: "CmdOrCtrl+Shift+Space"}
	svc := NewSettings(Dependencies{QuickChatShortcuts: controller})
	_, err := svc.SaveShortcutSettings(context.Background(), settings_dto.SaveShortcutSettingsInput{QuickChatShortcut: "CmdOrCtrl+Shift+K"})
	if err != nil {
		t.Fatal(err)
	}

	bytes, err := os.ReadFile(filepath.Join(tempDataDir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg data_models.Config
	if err := json.Unmarshal(bytes, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.QuickChatShortcut != "CmdOrCtrl+Shift+K" {
		t.Fatalf("quick_chat_shortcut = %q, want CmdOrCtrl+Shift+K", cfg.QuickChatShortcut)
	}
	if len(controller.updated) != 1 || controller.updated[0] != "CmdOrCtrl+Shift+K" {
		t.Fatalf("updated shortcuts = %v, want [CmdOrCtrl+Shift+K]", controller.updated)
	}
}

// TestBuildDataDirectoryItemsDarwin covers the macOS listing, including WebKit stores outside the data dir.
func TestBuildDataDirectoryItemsDarwin(t *testing.T) {
	dataDir := filepath.Join("/", "Users", "u", ".fire-poker")
	home := filepath.Join("/", "Users", "u")
	tempDir := filepath.Join("/", "tmp", "fire-poker")

	items := buildDataDirectoryItems(dataDir, "darwin", home, tempDir)
	ids := make([]string, 0, len(items))
	byID := map[string]settings_dto.DataDirectoryItem{}
	for _, item := range items {
		ids = append(ids, item.ID)
		byID[item.ID] = item
	}
	wantIDs := []string{"data_root", "config", "database", "sessions", "skills", "plugins", "logs", "cache", "webview_data", "webview_cache", "temp"}
	if len(ids) != len(wantIDs) {
		t.Fatalf("ids = %v, want %v", ids, wantIDs)
	}
	for i := range wantIDs {
		if ids[i] != wantIDs[i] {
			t.Fatalf("ids = %v, want %v", ids, wantIDs)
		}
	}

	config := byID["config"]
	if !config.IsFile || config.Path != filepath.Join(dataDir, "config.json") || config.RevealDir != dataDir {
		t.Fatalf("unexpected config item: %+v", config)
	}
	cache := byID["cache"]
	if cache.IsFile || cache.Path != filepath.Join(dataDir, "cache") || cache.RevealDir != cache.Path {
		t.Fatalf("unexpected cache item: %+v", cache)
	}
	if got := byID["webview_data"].Path; got != filepath.Join(home, "Library", "WebKit", dir.BundleIdentifier) {
		t.Fatalf("unexpected webview data path: %q", got)
	}
}

// TestBuildDataDirectoryItemsOtherPlatforms keeps WebView stores out of the listing where they live inside the cache.
func TestBuildDataDirectoryItemsOtherPlatforms(t *testing.T) {
	for _, goos := range []string{"windows", "linux"} {
		items := buildDataDirectoryItems(filepath.Join("/", "d"), goos, filepath.Join("/", "h"), filepath.Join("/", "t"))
		for _, item := range items {
			if item.ID == "webview_data" || item.ID == "webview_cache" {
				t.Fatalf("%s must not list %s", goos, item.ID)
			}
		}
		if last := items[len(items)-1]; last.ID != "temp" {
			t.Fatalf("%s: temp should be listed last, got %q", goos, last.ID)
		}
	}
}

// TestListDataDirectoriesReportsExistence marks only paths that exist on disk.
func TestListDataDirectoriesReportsExistence(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("FIRE_POKER_DATA_DIR", dataDir)
	if err := os.WriteFile(filepath.Join(dataDir, "config.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := newTestSettingsService(t).ListDataDirectories(context.Background(), settings_dto.ListDataDirectoriesInput{})
	if err != nil {
		t.Fatal(err)
	}
	exists := map[string]bool{}
	for _, item := range out.Items {
		exists[item.ID] = item.Exists
	}
	if !exists["data_root"] || !exists["config"] {
		t.Fatalf("data root and config should exist: %v", exists)
	}
	if exists["database"] || exists["logs"] || exists["cache"] {
		t.Fatalf("untouched paths should be reported missing: %v", exists)
	}
}
