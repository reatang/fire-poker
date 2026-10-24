package plugin_dto

import "github.com/reatang/fire-poker/backend/models/data_models"

// UpdateCliManifestInput carries the edited manifest JSON text for a specific CLI plugin.
type UpdateCliManifestInput struct {
	ID           string `json:"id"`
	ManifestText string `json:"manifest_text"`
}

// UpdateCliManifestOutput returns the updated ExtensionItem after manifest validation + save.
type UpdateCliManifestOutput struct {
	Extension data_models.ExtensionItem `json:"extension"`
}
