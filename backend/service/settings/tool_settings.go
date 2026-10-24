package settings

import (
	"context"
	"encoding/json"

	"github.com/reatang/fire-poker/backend/pkg/agent/toolkit"
	"github.com/reatang/fire-poker/backend/pkg/agent/tools"
	"github.com/reatang/fire-poker/backend/pkg/configfile"
	"github.com/reatang/fire-poker/backend/pkg/ierror"
	"github.com/reatang/fire-poker/backend/service/settings/settings_dto"
)

// SaveToolSettings persists application defaults without updating any existing session.
func (s *Settings) SaveToolSettings(ctx context.Context, input settings_dto.SaveToolSettingsInput) (*settings_dto.SaveToolSettingsOutput, error) {
	config, err := s.loadConfig()
	if err != nil {
		return nil, ierror.Error(ierror.ErrSettingsLoadConfig, err)
	}
	err = configfile.Update(config.DataDir, func(current map[string]json.RawMessage) error {
		var previous []string
		if raw, ok := current["default_tool_ids"]; ok {
			if err := json.Unmarshal(raw, &previous); err != nil {
				return err
			}
		}
		if previous == nil {
			previous = tools.DefaultToolIDs()
		}
		if s.sessionToolDefaults != nil {
			if err := s.sessionToolDefaults.InitializeSessionToolDefaults(previous); err != nil {
				return err
			}
		}
		data, err := json.Marshal(toolkit.NormalizeToolIDs(input.DefaultToolIDs))
		if err != nil {
			return err
		}
		current["default_tool_ids"] = data
		return nil
	})
	if err != nil {
		return nil, ierror.Error(ierror.ErrSettingsSaveConfig, err)
	}
	config, err = s.loadConfig()
	if err != nil {
		return nil, ierror.Error(ierror.ErrSettingsLoadConfig, err)
	}
	return &settings_dto.SaveToolSettingsOutput{DefaultToolIDs: config.DefaultToolIDs}, nil
}
