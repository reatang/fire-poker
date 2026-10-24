package agent_dto

// SessionToolsInput identifies a session whose tool selection should be loaded.
type SessionToolsInput struct {
	SessionID uint `json:"session_id"`
}

// SaveSessionToolsInput changes only the selected session's tools.
type SaveSessionToolsInput struct {
	SessionID uint     `json:"session_id"`
	ToolIDs   []string `json:"tool_ids"`
}

// SessionToolsOutput returns a concrete selection; an empty array means all selectable tools are off.
type SessionToolsOutput struct {
	ToolIDs []string `json:"tool_ids"`
	Locked  bool     `json:"locked"`
}
