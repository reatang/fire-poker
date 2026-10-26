package agent_dto

// SendMessageInput selects the provider and model by identity; the backend resolves credentials.
type SendMessageInput struct {
	RegenerateMessageID uint              `json:"regenerate_message_id,omitempty"`
	SessionID           uint              `json:"session_id"`
	Content             string            `json:"content"`
	SystemPrompt        string            `json:"system_prompt,omitempty"`
	SkillName           string            `json:"skill_name,omitempty"`
	ProviderID          uint              `json:"provider_id"`
	ModelName           string            `json:"model_name"`
	EnabledUserTools    []string          `json:"enabled_user_tools"`
	Attachments         []AttachmentInput `json:"attachments,omitempty"`
}

type SendMessageOutput struct {
}
