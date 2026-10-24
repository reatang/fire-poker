package agent

import (
	"context"
	"github.com/reatang/fire-poker/backend/models/data_models"
	"github.com/reatang/fire-poker/backend/service/agent/agent_dto"
	"testing"
)

func TestForkSessionPreservesLockedPrompt(t *testing.T) {
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	stor := newTestStorage(t)
	svc := NewAgent(stor)
	source, _ := stor.CreateSession(data_models.Session{Title: "fork service", SystemPrompt: "fixed", PromptName: "test prompt"})
	msg, _ := stor.CreateMessage(data_models.Message{SessionID: source.ID, Role: "user", ContentType: "text", Content: "q"})
	out, err := svc.ForkSession(context.Background(), agent_dto.ForkSessionInput{SessionID: source.ID, MessageID: msg.ID})
	if err != nil {
		t.Fatal(err)
	}
	prompt, err := svc.GetSessionPrompt(context.Background(), agent_dto.SessionPromptInput{SessionID: out.Session.ID})
	if err != nil || prompt.System != "fixed" || !prompt.Locked {
		t.Fatalf("fork prompt changed: %+v %v", prompt, err)
	}
	if _, err := svc.ForkSession(context.Background(), agent_dto.ForkSessionInput{}); err == nil {
		t.Fatal("missing boundary accepted")
	}
}
