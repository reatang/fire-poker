package storage

import (
	"github.com/reatang/fire-poker/backend/models/data_models"
	"testing"
)

func TestBrandRenamePinsOnlyStartedImplicitDefaults(t *testing.T) {
	s := newTestStorage(t)
	started, _ := s.CreateSession(data_models.Session{Title: "old"})
	custom, _ := s.CreateSession(data_models.Session{Title: "custom", SystemPrompt: "user instruction", PromptName: "mine"})
	fresh, _ := s.CreateSession(data_models.Session{Title: "empty"})
	for _, id := range []uint{started.ID, custom.ID} {
		s.CreateMessage(data_models.Message{SessionID: id, Role: "user", ContentType: "text", Content: "hello"})
	}
	if err := s.PinLegacyDefaultPrompts("original default"); err != nil {
		t.Fatal(err)
	}
	old, _ := s.GetSession(started.ID)
	user, _ := s.GetSession(custom.ID)
	empty, _ := s.GetSession(fresh.ID)
	if old.SystemPrompt != "original default" || user.SystemPrompt != "user instruction" || empty.SystemPrompt != "" {
		t.Fatal("rename changed the wrong prompt")
	}
	if !old.UpdatedAt.Equal(started.UpdatedAt) {
		t.Fatal("rename changed conversation recency")
	}
	if err := s.PinLegacyDefaultPrompts("another default"); err != nil {
		t.Fatal(err)
	}
	again, _ := s.GetSession(started.ID)
	if again.SystemPrompt != "original default" {
		t.Fatal("fixed default changed on repeated startup")
	}
}
