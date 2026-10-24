package plugin_dto

import "github.com/reatang/fire-poker/backend/models/data_models"

type ReloadExtensionInput struct {
	ID string `json:"id"`
}

type ReloadExtensionOutput struct {
	Extension data_models.ExtensionItem `json:"extension"`
}
