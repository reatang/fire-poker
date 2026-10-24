package plugin_dto

import "github.com/reatang/fire-poker/backend/models/data_models"

type ImportExtensionInput struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

type ImportExtensionOutput struct {
	Extension data_models.ExtensionItem `json:"extension"`
}
