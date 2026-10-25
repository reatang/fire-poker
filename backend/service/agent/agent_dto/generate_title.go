package agent_dto

// GenerateTitleInput selects the session and model; credentials are resolved on the backend.
type GenerateTitleInput struct {
	SessionID  uint   `json:"session_id"`
	ProviderID uint   `json:"provider_id"`
	ModelName  string `json:"model_name"`
}

type GenerateTitleOutput struct {
	Title string `json:"title"`
}
