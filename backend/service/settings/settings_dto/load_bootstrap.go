package settings_dto

import "github.com/reatang/fire-poker/backend/models/view_model"

type LoadBootstrapInput struct {
}

type LoadBootstrapOutput struct {
	Bootstrap view_model.SettingsBootstrap `json:"bootstrap"`
}
