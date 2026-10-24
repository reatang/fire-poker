package agent

import (
	"context"
	"fmt"
	"github.com/reatang/fire-poker/backend/models/data_models"
	"testing"
	"time"
)

func TestForkContinuesWithEntireCopiedHistory(t *testing.T) {
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	m := NewManager(newTestStorage(t))
	source, err := m.Storage().CreateSession(data_models.Session{Title: "long source", SystemPrompt: "fixed branch instruction"})
	if err != nil {
		t.Fatal(err)
	}
	messages := make([]data_models.Message, 1002)
	for i := range messages {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		messages[i] = data_models.Message{SessionID: source.ID, Role: role, ContentType: "text", Content: fmt.Sprintf("message %d", i)}
	}
	if err := m.Storage().CreateMessages(&messages); err != nil {
		t.Fatal(err)
	}
	target := messages[len(messages)-1]
	if _, err := m.Storage().CreateMessage(data_models.Message{SessionID: source.ID, Role: "user", ContentType: "text", Content: "future source message"}); err != nil {
		t.Fatal(err)
	}
	fork, err := m.Storage().ForkSession(source.ID, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	r := newDelayedRunner()
	m.runners["http://test/branch"] = r
	if err := NewChatHandler(m).SendMessage(context.Background(), SendMessageParams{SessionID: fork.ID, Content: "continue in branch", BaseURL: "http://test", ModelName: "branch"}); err != nil {
		t.Fatal(err)
	}
	if len(r.lastRunOpts.Messages) != 1002 {
		t.Fatalf("branch context was truncated: %d", len(r.lastRunOpts.Messages))
	}
	if r.lastRunOpts.Messages[1001].Content != "message 1001" || r.lastMessage.Content != "continue in branch" {
		t.Fatal("wrong branch boundary")
	}
	if r.lastRunOpts.Instruction != "fixed branch instruction" {
		t.Fatal("branch instruction changed")
	}
	close(r.events)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := m.Streams().StopAndWait(ctx, fork.ID); err != nil {
		t.Fatal(err)
	}
}
