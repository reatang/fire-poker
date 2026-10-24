package agent

import (
	"github.com/reatang/fire-poker/backend/pkg/id/prompt_id"
	"github.com/reatang/fire-poker/backend/pkg/prompt"
	"strings"
)

// DefaultSystemPrompt returns the same default used by the main agent.
func DefaultSystemPrompt() string {
	value, err := prompt.Load(prompt_id.MainAgent)
	if err != nil || strings.TrimSpace(value) == "" {
		return defaultMainAgentPrompt
	}
	return value
}
