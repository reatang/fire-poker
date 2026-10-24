package storage

import (
	"fmt"
	"strings"

	"github.com/reatang/fire-poker/backend/models/data_models"
	"gorm.io/gorm"
)

// ForkSession copies the persisted prefix, including the selected text message,
// in one transaction. New message IDs and parent links are independent.
func (s *Storage) ForkSession(sessionID, messageID uint) (*data_models.Session, error) {
	var fork data_models.Session
	err := s.sqliteDB.Transaction(func(tx *gorm.DB) error {
		var source data_models.Session
		if err := tx.First(&source, sessionID).Error; err != nil {
			return err
		}
		var boundary data_models.Message
		if err := tx.Where("id = ? AND session_id = ?", messageID, sessionID).First(&boundary).Error; err != nil {
			return err
		}
		if boundary.ContentType != "text" || (boundary.Role != "user" && boundary.Role != "assistant") {
			return fmt.Errorf("only a saved chat message can be forked")
		}
		var messages []data_models.Message
		if err := tx.Where("session_id = ? AND id <= ?", sessionID, messageID).Order("id ASC").Find(&messages).Error; err != nil {
			return err
		}
		title := []rune(strings.TrimSpace(source.Title))
		if len(title) > 200 {
			title = title[:200]
		}
		fork = data_models.Session{Title: string(title) + " · 分叉", Kind: "user", Status: "idle", Tags: "[]", SystemPrompt: source.SystemPrompt, PromptName: source.PromptName, ToolIDs: source.ToolIDs}
		if err := tx.Create(&fork).Error; err != nil {
			return err
		}
		ids := make(map[uint]uint, len(messages))
		for _, original := range messages {
			copied := original
			copied.OrmModel = data_models.OrmModel{CreatedAt: original.CreatedAt, UpdatedAt: original.UpdatedAt}
			copied.SessionID = fork.ID
			copied.ParentID = nil
			if original.ParentID != nil {
				if id, ok := ids[*original.ParentID]; ok {
					copied.ParentID = &id
				}
			}
			if err := tx.Create(&copied).Error; err != nil {
				return err
			}
			ids[original.ID] = copied.ID
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &fork, nil
}
