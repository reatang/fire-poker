package provider_dto

import (
	"github.com/reatang/fire-poker/backend/models/view_model"
	"github.com/reatang/fire-poker/backend/pkg/provider"
)

type CreateProviderInput struct {
	ProviderName string             `json:"provider_name"` // 供应商名称
	ProviderType provider.Type      `json:"provider_type"` // 供应商类型
	BaseUrl      string             `json:"base_url"`      // 供应商基础URL
	ApiKey       string             `json:"api_key"`       // 供应商API密钥
	Enable       bool               `json:"enable"`        // 是否启用
	DefaultModel *string            `json:"default_model"` // 默认模型
	Models       []view_model.Model `json:"models"`        // 供应商模型列表
}

type CreateProviderOutput struct {
}
