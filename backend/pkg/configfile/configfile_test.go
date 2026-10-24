package configfile

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/reatang/fire-poker/backend/models/data_models"
)

// TestStaleModuleSnapshotsCannotRestoreTools reproduces a delayed plugin refresh after Apply.
func TestStaleModuleSnapshotsCannotRestoreTools(t *testing.T) {
	dataDir := t.TempDir()
	original := &data_models.Config{DataDir: dataDir, DefaultToolIDs: []string{"shell"}, Language: "zh-CN"}
	if err := SaveFields(dataDir, original); err != nil {
		t.Fatal(err)
	}
	if err := SaveFields(dataDir, &data_models.Config{DefaultToolIDs: []string{}}, "default_tool_ids"); err != nil {
		t.Fatal(err)
	}
	original.Extensions = []data_models.ExtensionItem{{ID: "mcp:test", Enabled: true}}
	if err := SaveFields(dataDir, original, "extensions"); err != nil {
		t.Fatal(err)
	}
	if err := SaveFields(dataDir, original, "font_size"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dataDir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved data_models.Config
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.DefaultToolIDs == nil || len(saved.DefaultToolIDs) != 0 {
		t.Fatalf("stale writer restored tools: %s", raw)
	}
	if len(saved.Extensions) != 1 || saved.Language != "zh-CN" {
		t.Fatalf("unrelated config lost: %s", raw)
	}
}

// TestConcurrentReadersSeeCompleteConfig verifies atomic replacement never exposes truncated JSON.
func TestConcurrentReadersSeeCompleteConfig(t *testing.T) {
	dataDir := t.TempDir()
	if err := SaveFields(dataDir, &data_models.Config{}); err != nil {
		t.Fatal(err)
	}
	done, stopped := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		for {
			select {
			case <-done:
				return
			default:
			}
			raw, err := ReadFile(filepath.Join(dataDir, "config.json"))
			if err != nil {
				t.Error(err)
				return
			}
			if !json.Valid(raw) {
				t.Errorf("partial config: %s", raw)
				return
			}
		}
	}()
	for i := 0; i < 20; i++ {
		if err := SaveFields(dataDir, &data_models.Config{DefaultToolIDs: []string{}}, "default_tool_ids"); err != nil {
			t.Error(err)
			break
		}
	}
	close(done)
	<-stopped
}

// TestCrossProcessConfigUpdates checks the OS lock used by multiple portable app instances.
func TestCrossProcessConfigUpdates(t *testing.T) {
	dataDir := t.TempDir()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var workers []*exec.Cmd
	for i := 0; i < 3; i++ {
		worker := exec.Command(executable, "-test.run=^TestConfigWriterProcess$")
		worker.Env = append(os.Environ(), "FIRE_POKER_CONFIG_TEST_DIR="+dataDir)
		if err := worker.Start(); err != nil {
			t.Fatal(err)
		}
		workers = append(workers, worker)
	}
	for _, worker := range workers {
		if err := worker.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(dataDir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]int
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	if result["counter"] != 30 {
		t.Fatalf("cross-process updates lost: %s", raw)
	}
}

// TestConfigWriterProcess is the isolated writer invoked by the cross-process regression.
func TestConfigWriterProcess(t *testing.T) {
	dataDir := os.Getenv("FIRE_POKER_CONFIG_TEST_DIR")
	if dataDir == "" {
		return
	}
	for i := 0; i < 10; i++ {
		if err := Update(dataDir, func(current map[string]json.RawMessage) error {
			var count int
			if raw := current["counter"]; raw != nil {
				if err := json.Unmarshal(raw, &count); err != nil {
					return err
				}
			}
			current["counter"], _ = json.Marshal(count + 1)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}

// TestConcurrentUpdatesAreSerialized verifies the lock covers read, mutation and commit together.
func TestConcurrentUpdatesAreSerialized(t *testing.T) {
	dataDir := t.TempDir()
	var workers sync.WaitGroup
	for i := 0; i < 30; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if err := Update(dataDir, func(current map[string]json.RawMessage) error {
				var count int
				if raw := current["counter"]; raw != nil {
					if err := json.Unmarshal(raw, &count); err != nil {
						return err
					}
				}
				current["counter"], _ = json.Marshal(count + 1)
				return nil
			}); err != nil {
				t.Error(err)
			}
		}()
	}
	workers.Wait()
	raw, err := os.ReadFile(filepath.Join(dataDir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved map[string]int
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if saved["counter"] != 30 {
		t.Fatalf("lost concurrent update: %s", raw)
	}
}

// TestInvalidConfigIsNotOverwritten ensures persistence errors do not silently reset defaults.
func TestInvalidConfigIsNotOverwritten(t *testing.T) {
	dataDir := t.TempDir()
	path := filepath.Join(dataDir, "config.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SaveFields(dataDir, &data_models.Config{}, "default_tool_ids"); err == nil {
		t.Fatal("accepted invalid config")
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "{broken" {
		t.Fatal("invalid config was overwritten")
	}
}
