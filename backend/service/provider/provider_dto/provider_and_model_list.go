package provider_dto

import "github.com/reatang/fire-poker/backend/models/view_model"

type ProviderAndModelListInput struct {
}

type ProviderAndModelListOutput struct {
	ProviderModels []view_model.ProviderModel `json:"provider_models"`
}
