package plugin_dto

import "github.com/reatang/fire-poker/backend/models/data_models"

type GetExtensionDetailInput struct {
	ID string `json:"id"`
}

type GetExtensionDetailOutput struct {
	Extension  data_models.ExtensionItem `json:"extension"`
	ConfigText string                    `json:"config_text"`
}
