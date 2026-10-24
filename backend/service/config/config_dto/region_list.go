package config_dto

import "github.com/reatang/fire-poker/backend/models/view_model"

type RegionListInput struct {
}

type RegionListOutput struct {
	Regions []view_model.Region `json:"regions"`
}
