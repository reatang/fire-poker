package config_dto

import "github.com/reatang/fire-poker/backend/models/view_model"

type LanguageListInput struct {
}

type LanguageListOutput struct {
	Languages []view_model.Language `json:"languages"`
}
