package agent_dto

import "github.com/reatang/fire-poker/backend/pkg/provider"

type SendMessageInput struct {
	RegenerateMessageID uint              `json:"regenerate_message_id,omitempty"`
	SessionID           uint              `json:"session_id"`
	Content             string            `json:"content"`
	SystemPrompt        string            `json:"system_prompt,omitempty"`
	SkillName           string            `json:"skill_name,omitempty"`
	BaseURL             string            `json:"base_url"`
	ApiKey              string            `json:"api_key"`
	ModelName           string            `json:"model_name"`
	ProviderType        provider.Type     `json:"provider_type"`
	EnabledUserTools    []string          `json:"enabled_user_tools"`
	Attachments         []AttachmentInput `json:"attachments,omitempty"`
}

type SendMessageOutput struct {
}
