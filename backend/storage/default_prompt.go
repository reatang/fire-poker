package storage

import "github.com/reatang/fire-poker/backend/models/data_models"

// PinLegacyDefaultPrompts preserves the original instruction in started chats.
// New chats snapshot the current default before saving their first question.
func (s *Storage) PinLegacyDefaultPrompts(original string) error {
	return s.sqliteDB.Model(&data_models.Session{}).
		Where("TRIM(system_prompt) = '' OR system_prompt IS NULL").
		Where("EXISTS (SELECT 1 FROM messages WHERE messages.session_id = sessions.id)").
		UpdateColumns(map[string]any{"system_prompt": original, "prompt_name": "默认系统提示词"}).Error
}
