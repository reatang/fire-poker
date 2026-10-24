package view_model

import "github.com/reatang/fire-poker/backend/pkg/provider"

// Provider is the provider list/detail payload returned to the frontend.
type Provider struct {
	ID        uint          `json:"id"`
	Icon      string        `json:"icon"`
	Type      provider.Type `json:"provider_type"`
	Name      string        `json:"provider_name"`
	BaseURL   string        `json:"base_url"`
	Enabled   bool          `json:"enabled"`
	ApiKey    string        `json:"api_key"`
	IsDefault bool          `json:"is_default"` // 是否是默认供应商
}
