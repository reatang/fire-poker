package storage

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/reatang/fire-poker/backend/models/data_models"
	"github.com/reatang/fire-poker/backend/pkg/agent/toolkit"
)

// ErrSessionToolsLocked indicates that the conversation's tool selection is immutable.
var ErrSessionToolsLocked = errors.New("对话开始后不能修改工具选择，请新建对话")

// UpdateSessionTools saves an explicit selection only before any message exists, including archived messages.
func (s *Storage) UpdateSessionTools(id uint, ids []string) error {
	data, err := json.Marshal(toolkit.NormalizeToolIDs(ids))
	if err != nil {
		return err
	}
	result := s.sqliteDB.Model(&data_models.Session{}).Where("id = ?", id).
		Where("NOT EXISTS (SELECT 1 FROM messages WHERE messages.session_id = ?)", id).
		UpdateColumn("tool_ids", string(data))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		if locked, err := s.SessionToolsLocked(id); err != nil {
			return err
		} else if locked {
			return ErrSessionToolsLocked
		}
		return fmt.Errorf("session %d not found", id)
	}
	return nil
}

// SessionToolsLocked includes archived messages so a started conversation cannot be unlocked by deleting history.
func (s *Storage) SessionToolsLocked(id uint) (bool, error) {
	var count int64
	err := s.sqliteDB.Unscoped().Model(&data_models.Message{}).Where("session_id = ?", id).Count(&count).Error
	return count > 0, err
}

// InitializeSessionTools freezes defaults only if the session has no saved selection yet.
func (s *Storage) InitializeSessionTools(id uint, ids []string) error {
	data, err := json.Marshal(toolkit.NormalizeToolIDs(ids))
	if err != nil {
		return err
	}
	return s.sqliteDB.Model(&data_models.Session{}).Where("id = ? AND (tool_ids = '' OR tool_ids IS NULL)", id).UpdateColumn("tool_ids", string(data)).Error
}

// InitializeSessionToolDefaults snapshots old defaults for unmigrated sessions without changing saved selections.
func (s *Storage) InitializeSessionToolDefaults(ids []string) error {
	data, err := json.Marshal(toolkit.NormalizeToolIDs(ids))
	if err != nil {
		return err
	}
	return s.sqliteDB.Model(&data_models.Session{}).Where("tool_ids = '' OR tool_ids IS NULL").UpdateColumn("tool_ids", string(data)).Error
}
