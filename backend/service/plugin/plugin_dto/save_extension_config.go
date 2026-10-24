package plugin_dto

import "github.com/reatang/fire-poker/backend/models/data_models"

type SaveExtensionConfigInput struct {
	ID         string `json:"id"`
	ConfigText string `json:"config_text"`
}

type SaveExtensionConfigOutput struct {
	Extension data_models.ExtensionItem `json:"extension"`
}
