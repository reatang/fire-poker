package agent_dto

// SpawnTaskSessionInput describes the initial prompt for an automated task session.
type SpawnTaskSessionInput struct {
	Title        string `json:"title"`
	SystemPrompt string `json:"system_prompt"`
	UserMessage  string `json:"user_message"`
	SkillName    string `json:"skill_name"`
	ProviderID   uint   `json:"provider_id"`
	ModelName    string `json:"model_name"`
}

// SpawnTaskSessionOutput returns the created task session identity.
type SpawnTaskSessionOutput struct {
	SessionID uint   `json:"session_id"`
	Title     string `json:"title"`
}
