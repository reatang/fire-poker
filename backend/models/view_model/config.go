package view_model

import "github.com/reatang/fire-poker/backend/pkg/provider"

type Language struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Region struct {
	Icon string `json:"icon"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

type SupportedProvider struct {
	Type        provider.Type `json:"type"`
	Icon        string        `json:"icon"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	BaseURL     string        `json:"base_url"`
}
