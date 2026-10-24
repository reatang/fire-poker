package settings_dto

// SaveToolSettingsInput changes defaults for future sessions only.
type SaveToolSettingsInput struct {
	DefaultToolIDs []string `json:"default_tool_ids"`
}

// SaveToolSettingsOutput returns the normalized persisted defaults.
type SaveToolSettingsOutput struct {
	DefaultToolIDs []string `json:"default_tool_ids"`
}
