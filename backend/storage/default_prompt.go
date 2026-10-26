package storage

import (
	"github.com/reatang/fire-poker/backend/models/data_models"
	"github.com/reatang/fire-poker/backend/pkg/i18n"
)

// Prompt name keys localize the label snapshotted with a session's system prompt.
// The label is stored in the language active when the chat started.
const (
	PromptNameDefaultKey = "session.prompt_name.default"
	PromptNameCustomKey  = "session.prompt_name.custom"
)

// PinLegacyDefaultPrompts preserves the original instruction in started chats.
// New chats snapshot the current default before saving their first question.
func (s *Storage) PinLegacyDefaultPrompts(original string) error {
	return s.sqliteDB.Model(&data_models.Session{}).
		Where("TRIM(system_prompt) = '' OR system_prompt IS NULL").
		Where("EXISTS (SELECT 1 FROM messages WHERE messages.session_id = sessions.id)").
		UpdateColumns(map[string]any{"system_prompt": original, "prompt_name": i18n.TCurrent(PromptNameDefaultKey, nil)}).Error
}
