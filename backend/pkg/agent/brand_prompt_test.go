package agent

import (
	"context"
	"github.com/reatang/fire-poker/backend/models/data_models"
	"testing"
	"time"
)

func TestFirstQuestionSnapshotsCurrentBrandDefault(t *testing.T) {
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	m := NewManager(newTestStorage(t))
	s, err := m.Storage().CreateSession(data_models.Session{Title: "new brand chat"})
	if err != nil {
		t.Fatal(err)
	}
	r := newDelayedRunner()
	m.runners["http://test/brand"] = r
	if err := NewChatHandler(m).SendMessage(context.Background(), SendMessageParams{SessionID: s.ID, Content: "hello", BaseURL: "http://test", ModelName: "brand"}); err != nil {
		t.Fatal(err)
	}
	saved, err := m.Storage().GetSession(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.SystemPrompt != DefaultSystemPrompt() || r.lastRunOpts.Instruction != saved.SystemPrompt {
		t.Fatal("current default was not snapshotted")
	}
	close(r.events)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := m.Streams().StopAndWait(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
}
