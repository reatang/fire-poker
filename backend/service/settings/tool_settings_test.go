package settings

import (
	"context"
	"encoding/json"
	"github.com/reatang/fire-poker/backend/models/data_models"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/reatang/fire-poker/backend/service/settings/settings_dto"
)

type toolDefaultsRecorder struct{ snapshots [][]string }

// InitializeSessionToolDefaults records which prior defaults legacy sessions receive.
func (r *toolDefaultsRecorder) InitializeSessionToolDefaults(ids []string) error {
	r.snapshots = append(r.snapshots, append([]string{}, ids...))
	return nil
}

// TestToolSettingsUseActivePortableDirectory ensures copied configs cannot redirect saves to a previous release.
func TestToolSettingsUseActivePortableDirectory(t *testing.T) {
	activeDir, oldDir := t.TempDir(), t.TempDir()
	t.Setenv("FIRE_POKER_DATA_DIR", activeDir)
	raw, err := json.Marshal(data_models.Config{DataDir: oldDir, DefaultToolIDs: []string{"shell"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(activeDir, "config.json"), filepath.Join(oldDir, "config.json")} {
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s := NewSettings(Dependencies{})
	result, err := s.SaveToolSettings(context.Background(), settings_dto.SaveToolSettingsInput{DefaultToolIDs: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if result.DefaultToolIDs == nil || len(result.DefaultToolIDs) != 0 {
		t.Fatalf("Apply restored old selection: %v", result.DefaultToolIDs)
	}
	reopened := NewSettings(Dependencies{})
	loaded, err := reopened.LoadBootstrap(context.Background(), settings_dto.LoadBootstrapInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Bootstrap.DefaultToolIDs) != 0 {
		t.Fatal("reopened settings restored tools")
	}
	old, err := os.ReadFile(filepath.Join(oldDir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(old) != string(raw) {
		t.Fatal("save modified inactive directory")
	}
}

// TestToolDefaultsPersistExplicitEmptyAndPreservePriorDefaults checks config storage and legacy migration ordering.
func TestToolDefaultsPersistExplicitEmptyAndPreservePriorDefaults(t *testing.T) {
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	recorder := &toolDefaultsRecorder{}
	s := NewSettings(Dependencies{SessionToolDefaults: recorder})
	ctx := context.Background()
	if _, err := s.SaveToolSettings(ctx, settings_dto.SaveToolSettingsInput{DefaultToolIDs: []string{" web_fetch ", "web_fetch"}}); err != nil {
		t.Fatal(err)
	}
	bootstrap, err := s.LoadBootstrap(ctx, settings_dto.LoadBootstrapInput{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(bootstrap.Bootstrap.DefaultToolIDs, []string{"web_fetch"}) {
		t.Fatalf("defaults: %v", bootstrap.Bootstrap.DefaultToolIDs)
	}
	if _, err := s.SaveToolSettings(ctx, settings_dto.SaveToolSettingsInput{DefaultToolIDs: []string{}}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(recorder.snapshots[1], []string{"web_fetch"}) {
		t.Fatal("legacy sessions received the new defaults instead of old defaults")
	}
	reopened := NewSettings(Dependencies{})
	bootstrap, err = reopened.LoadBootstrap(ctx, settings_dto.LoadBootstrapInput{})
	if err != nil {
		t.Fatal(err)
	}
	if bootstrap.Bootstrap.DefaultToolIDs == nil || len(bootstrap.Bootstrap.DefaultToolIDs) != 0 {
		t.Fatal("explicit empty defaults reverted after reload")
	}
}
