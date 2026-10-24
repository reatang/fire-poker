package storage

import (
	"fmt"
	"github.com/reatang/fire-poker/backend/models/data_models"
)

// RegenerationTurn only accepts a response to the latest user question.
func (s *Storage) RegenerationTurn(sessionID, responseID uint) (*data_models.Message, error) {
	var response, question data_models.Message
	if err := s.sqliteDB.Where("id = ? AND session_id = ? AND role = ? AND content_type IN ?", responseID, sessionID, "assistant", []string{"text", "error"}).First(&response).Error; err != nil {
		return nil, err
	}
	if err := s.sqliteDB.Where("session_id = ? AND role = ? AND content_type = ?", sessionID, "user", "text").Order("id DESC").First(&question).Error; err != nil {
		return nil, err
	}
	if response.ID <= question.ID {
		return nil, fmt.Errorf("only the latest response can be regenerated")
	}
	var newer int64
	if err := s.sqliteDB.Model(&data_models.Message{}).Where("session_id = ? AND id > ? AND role = ? AND content_type IN ?", sessionID, responseID, "assistant", []string{"text", "error"}).Count(&newer).Error; err != nil {
		return nil, err
	}
	if newer > 0 {
		return nil, fmt.Errorf("response has changed; reload before regenerating")
	}
	return &question, nil
}

// ArchiveRegeneratedResponse keeps the original rows recoverable by soft delete.
func (s *Storage) ArchiveRegeneratedResponse(sessionID, questionID, responseID uint) error {
	return s.sqliteDB.Where("session_id = ? AND id > ? AND id <= ?", sessionID, questionID, responseID).Delete(&data_models.Message{}).Error
}

func (s *Storage) CreateMessage(msg data_models.Message) (*data_models.Message, error) {
	if err := s.sqliteDB.Create(&msg).Error; err != nil {
		return nil, err
	}
	return &msg, nil
}

func (s *Storage) CreateMessages(msgs *[]data_models.Message) error {
	if len(*msgs) == 0 {
		return nil
	}
	return s.sqliteDB.Create(msgs).Error
}

func (s *Storage) ListMessagesForSession(sessionID uint, offset int, limit int) ([]data_models.Message, error) {
	var msgs []data_models.Message
	if err := s.sqliteDB.Where("session_id = ?", sessionID).
		Order("created_at ASC").
		Order("id ASC").
		Offset(offset).Limit(limit).
		Find(&msgs).Error; err != nil {
		return nil, err
	}
	return msgs, nil
}

func (s *Storage) CountMessagesForSession(sessionID uint) (int64, error) {
	var count int64
	if err := s.sqliteDB.Model(&data_models.Message{}).Where("session_id = ?", sessionID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Storage) DeleteMessagesForSession(sessionID uint) error {
	return s.sqliteDB.Where("session_id = ?", sessionID).Delete(&data_models.Message{}).Error
}
