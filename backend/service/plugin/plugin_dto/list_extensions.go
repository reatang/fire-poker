package plugin_dto

import "github.com/reatang/fire-poker/backend/models/data_models"

type ListExtensionsInput struct{}

type ListExtensionsOutput struct {
	Extensions []data_models.ExtensionItem `json:"extensions"`
}
