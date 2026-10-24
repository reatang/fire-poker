package skills

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/reatang/fire-poker/backend/models/data_models"
	"github.com/reatang/fire-poker/backend/pkg/configfile"
	"github.com/reatang/fire-poker/backend/pkg/dir"
)

// loadConfig reads the app config from config.json in the data directory.
// Returns a sensible default when the file does not yet exist.
func (s *Skills) loadConfig() (*data_models.Config, error) {
	dataDir, err := dir.GetDataDir()
	if err != nil {
		return nil, err
	}
	configPath := filepath.Join(dataDir, dir.ConfigFileName)
	bytes, err := configfile.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &data_models.Config{DataDir: dataDir}, nil
		}
		return nil, err
	}
	cfg := &data_models.Config{}
	if err := json.Unmarshal(bytes, cfg); err != nil {
		return nil, err
	}
	cfg.DataDir = dataDir
	return cfg, nil
}

// saveConfig writes the app config to config.json in the data directory.
func (s *Skills) saveConfig(cfg *data_models.Config) error {
	return configfile.SaveFields(cfg.DataDir, cfg, "disabled_skills")
}

// renameDisabledSkill rewrites a disabled skill entry when a user skill is renamed.
func (s *Skills) renameDisabledSkill(oldName string, newName string) error {
	cfg, err := s.loadConfig()
	if err != nil {
		return err
	}
	changed := false
	for i, name := range cfg.DisabledSkills {
		if name == oldName {
			cfg.DisabledSkills[i] = newName
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return s.saveConfig(cfg)
}
