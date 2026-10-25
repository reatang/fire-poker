package skills

import (
	"github.com/reatang/fire-poker/backend/models/data_models"
	"github.com/reatang/fire-poker/backend/pkg/configfile"
)

// loadConfig reads the app config from config.json in the data directory.
// Returns a sensible default when the file does not yet exist.
func (s *Skills) loadConfig() (*data_models.Config, error) {
	return configfile.LoadCurrent()
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
