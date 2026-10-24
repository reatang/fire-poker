package config_dto

import "github.com/reatang/fire-poker/backend/models/view_model"

type SupportedProviderListInput struct {
}

type SupportedProviderListOutput struct {
	SupportedProviders []view_model.SupportedProvider `json:"supported_providers"`
}
