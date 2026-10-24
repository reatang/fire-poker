package storage

import (
	"github.com/reatang/fire-poker/backend/models/data_models"
	"testing"
)

func TestRegenerationRejectsOlderTurnsAndOtherSessions(t *testing.T) {
	s := newTestStorage(t)
	a, _ := s.CreateSession(data_models.Session{Title: "a"})
	b, _ := s.CreateSession(data_models.Session{Title: "b"})
	s.CreateMessage(data_models.Message{SessionID: a.ID, Role: "user", ContentType: "text", Content: "q1"})
	answer, _ := s.CreateMessage(data_models.Message{SessionID: a.ID, Role: "assistant", ContentType: "text", Content: "a1"})
	if _, err := s.RegenerationTurn(b.ID, answer.ID); err == nil {
		t.Fatal("cross-session retry accepted")
	}
	question, _ := s.CreateMessage(data_models.Message{SessionID: a.ID, Role: "user", ContentType: "text", Content: "q2"})
	if _, err := s.RegenerationTurn(a.ID, answer.ID); err == nil {
		t.Fatal("old turn retry accepted")
	}
	latest, _ := s.CreateMessage(data_models.Message{SessionID: a.ID, Role: "assistant", ContentType: "text", Content: "a2"})
	fresh, _ := s.CreateMessage(data_models.Message{SessionID: a.ID, Role: "assistant", ContentType: "tool_call", Content: "new run tool"})
	if err := s.ArchiveRegeneratedResponse(a.ID, question.ID, latest.ID); err != nil {
		t.Fatal(err)
	}
	messages, _ := s.ListMessagesForSession(a.ID, 0, 100)
	if len(messages) != 4 || messages[3].ID != fresh.ID {
		t.Fatalf("archived new run messages: %+v", messages)
	}
	var archived data_models.Message
	if err := s.sqliteDB.Unscoped().First(&archived, latest.ID).Error; err != nil || !archived.DeletedAt.Valid {
		t.Fatal("old response was not archived recoverably")
	}
}
