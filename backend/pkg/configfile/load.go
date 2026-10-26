package configfile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/reatang/fire-poker/backend/models/data_models"
	"github.com/reatang/fire-poker/backend/pkg/dir"
)

// ErrInvalidConfig marks a config.json that exists but cannot be parsed.
var ErrInvalidConfig = errors.New("invalid config file")

// Load is the single reader of config.json. It never creates the file; a missing file yields
// the normalized defaults so feature checks behave the same before and after first save.
//
// Normalization applied to every caller:
//   - DataDir is the resolved directory, not whatever was persisted (portable launch overrides win);
//   - a nil extension list becomes empty so callers can append and save it directly;
//   - memory is enabled unless the user explicitly stored memory.enabled.
func Load(dataDir string) (*data_models.Config, error) {
	raw, err := ReadFile(filepath.Join(dataDir, dir.ConfigFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return normalize(&data_models.Config{}, dataDir, false), nil
		}
		return nil, err
	}
	cfg := &data_models.Config{}
	if err := json.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("%w: parse %s: %w", ErrInvalidConfig, dir.ConfigFileName, err)
	}
	return normalize(cfg, dataDir, memoryEnabledIsSet(raw)), nil
}

// LoadCurrent loads the config from the application's resolved data directory.
func LoadCurrent() (*data_models.Config, error) {
	dataDir, err := dir.GetDataDir()
	if err != nil {
		return nil, err
	}
	return Load(dataDir)
}

// LoadOrInit persists defaults when no config file exists yet, then loads the result.
// Ensure is a no-op when another module created the file first, so defaults never overwrite user data.
func LoadOrInit(dataDir string, defaults *data_models.Config) (*data_models.Config, error) {
	if _, err := os.Stat(filepath.Join(dataDir, dir.ConfigFileName)); os.IsNotExist(err) {
		if err := Ensure(dataDir, defaults); err != nil {
			return nil, err
		}
	}
	return Load(dataDir)
}

// normalize applies the defaults every reader relies on.
func normalize(cfg *data_models.Config, dataDir string, memoryEnabledSet bool) *data_models.Config {
	cfg.DataDir = dataDir
	if cfg.Extensions == nil {
		cfg.Extensions = []data_models.ExtensionItem{}
	}
	if !memoryEnabledSet {
		cfg.Memory.Enabled = true
	}
	return cfg
}

// memoryEnabledIsSet reports whether the top-level memory object explicitly stores "enabled".
// It inspects the JSON structure, so the word "memory" elsewhere (e.g. an extension name) does not count.
func memoryEnabledIsSet(raw []byte) bool {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return false
	}
	var memory map[string]json.RawMessage
	if err := json.Unmarshal(top["memory"], &memory); err != nil || memory == nil {
		return false
	}
	_, ok := memory["enabled"]
	return ok
}
