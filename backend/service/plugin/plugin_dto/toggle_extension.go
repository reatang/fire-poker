package plugin_dto

import "github.com/reatang/fire-poker/backend/models/data_models"

type ToggleExtensionInput struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}

type ToggleExtensionOutput struct {
	Extension data_models.ExtensionItem `json:"extension"`
}
