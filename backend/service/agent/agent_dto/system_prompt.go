package agent_dto

type SessionPromptInput struct {
	SessionID uint `json:"session_id"`
}
type UpdateSessionPromptInput struct {
	SessionID uint   `json:"session_id"`
	System    string `json:"system"`
	Name      string `json:"name"`
	Reset     bool   `json:"reset"`
}
type SessionPromptOutput struct {
	System        string `json:"system"`
	Name          string `json:"name"`
	DefaultSystem string `json:"default_system"`
	IsDefault     bool   `json:"is_default"`
	Locked        bool   `json:"locked"`
}
type PromptLibraryInput struct {
	Name   string `json:"name"`
	System string `json:"system"`
}
type PromptLibraryOutput struct {
	Names []string `json:"names"`
}
type PromptOutput struct {
	Name   string `json:"name"`
	System string `json:"system"`
}
