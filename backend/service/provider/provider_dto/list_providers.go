package provider_dto

import "github.com/reatang/fire-poker/backend/models/view_model"

type ListProvidersInput struct {
}

type ListProvidersOutput struct {
	Providers []ProviderWrapper `json:"providers"`
}

type ProviderWrapper struct {
	view_model.Provider `json:"providers"`
	Models              []view_model.Model `json:"models"`
}
