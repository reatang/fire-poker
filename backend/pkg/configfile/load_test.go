package configfile

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/reatang/fire-poker/backend/models/data_models"
)

// writeConfig stores raw config.json content for a load test.
func writeConfig(t *testing.T, dataDir string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dataDir, "config.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMissingFileReturnsNormalizedDefaults(t *testing.T) {
	dataDir := t.TempDir()

	cfg, err := Load(dataDir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.DataDir != dataDir {
		t.Fatalf("expected resolved data dir, got %q", cfg.DataDir)
	}
	if cfg.Extensions == nil || len(cfg.Extensions) != 0 {
		t.Fatalf("expected empty non-nil extensions, got %#v", cfg.Extensions)
	}
	if !cfg.Memory.Enabled {
		t.Fatal("expected memory to default to enabled")
	}
	if _, err := os.Stat(filepath.Join(dataDir, "config.json")); !os.IsNotExist(err) {
		t.Fatalf("Load must not create the file, stat err = %v", err)
	}
}

// TestLoadMemoryDefault covers when an absent setting means enabled and when the user's choice wins.
func TestLoadMemoryDefault(t *testing.T) {
	cases := []struct {
		name    string
		content string
		enabled bool
	}{
		{name: "no memory key", content: `{"locale":"en-US"}`, enabled: true},
		{name: "null memory", content: `{"memory":null}`, enabled: true},
		{name: "memory without enabled", content: `{"memory":{}}`, enabled: true},
		{name: "explicitly enabled", content: `{"memory":{"enabled":true}}`, enabled: true},
		{name: "explicitly disabled", content: `{"memory":{"enabled":false}}`, enabled: false},
		// The old substring check treated any "memory" text as an explicit setting and disabled the feature.
		{name: "memory text elsewhere", content: `{"extensions":[{"id":"mcp:memory","name":"memory","tools":[]}]}`, enabled: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dataDir := t.TempDir()
			writeConfig(t, dataDir, tc.content)
			cfg, err := Load(dataDir)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if cfg.Memory.Enabled != tc.enabled {
				t.Fatalf("memory enabled = %v, want %v", cfg.Memory.Enabled, tc.enabled)
			}
		})
	}
}

func TestLoadOverridesStoredDataDirAndKeepsFields(t *testing.T) {
	dataDir := t.TempDir()
	writeConfig(t, dataDir, `{"data_dir":"/stale/elsewhere","default_provider_id":4,"disabled_skills":["a"],"extensions":null}`)

	cfg, err := Load(dataDir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.DataDir != dataDir {
		t.Fatalf("expected resolved data dir to win, got %q", cfg.DataDir)
	}
	if cfg.DefaultProviderID != 4 || len(cfg.DisabledSkills) != 1 || cfg.DisabledSkills[0] != "a" {
		t.Fatalf("fields lost: %+v", cfg)
	}
	if cfg.Extensions == nil {
		t.Fatal("expected null extensions to normalize to an empty list")
	}
}

func TestLoadRejectsInvalidJSON(t *testing.T) {
	dataDir := t.TempDir()
	writeConfig(t, dataDir, `{"locale":`)

	if _, err := Load(dataDir); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig, got %v", err)
	}
}

func TestLoadOrInitCreatesDefaultsOnlyWhenMissing(t *testing.T) {
	dataDir := t.TempDir()
	defaults := &data_models.Config{Locale: "zh-CN", FontSize: "md"}

	cfg, err := LoadOrInit(dataDir, defaults)
	if err != nil {
		t.Fatalf("load or init: %v", err)
	}
	if cfg.Locale != "zh-CN" || cfg.FontSize != "md" || cfg.DataDir != dataDir {
		t.Fatalf("expected defaults to be persisted and loaded, got %+v", cfg)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "config.json")); err != nil {
		t.Fatalf("expected config file to be created: %v", err)
	}

	writeConfig(t, dataDir, `{"locale":"en-US"}`)
	cfg, err = LoadOrInit(dataDir, defaults)
	if err != nil {
		t.Fatalf("load existing: %v", err)
	}
	if cfg.Locale != "en-US" || cfg.FontSize != "" {
		t.Fatalf("existing config must not be overwritten by defaults, got %+v", cfg)
	}
}
