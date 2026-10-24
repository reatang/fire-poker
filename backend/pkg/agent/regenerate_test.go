package agent

import (
	"context"
	"fmt"
	"github.com/reatang/fire-poker/backend/models/data_models"
	"testing"
	"time"
	agentpkg "trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

type failingRegenerationRunner struct{ *delayedRunner }

func (r *failingRegenerationRunner) Run(context.Context, string, string, model.Message, ...agentpkg.RunOption) (<-chan *event.Event, error) {
	return nil, fmt.Errorf("runner unavailable")
}

func TestRegenerationReplaysPrefixAndDoesNotDuplicateQuestion(t *testing.T) {
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	m := NewManager(newTestStorage(t))
	s, _ := m.Storage().CreateSession(data_models.Session{Title: "regen", SystemPrompt: "fixed instruction"})
	for _, item := range []struct{ role, kind, content string }{{"user", "text", "previous question"}, {"assistant", "text", "previous answer"}, {"user", "text", "original question"}, {"assistant", "thinking", "old thinking"}, {"assistant", "text", "old answer"}} {
		if _, err := m.Storage().CreateMessage(data_models.Message{SessionID: s.ID, Role: item.role, ContentType: item.kind, Content: item.content}); err != nil {
			t.Fatal(err)
		}
	}
	before, _ := m.Storage().ListMessagesForSession(s.ID, 0, 100)
	target := before[len(before)-1]
	r := newDelayedRunner()
	injectSessionRunner(t, m, s.ID, "http://test", "model", "openai_compatibility", r)
	h := NewChatHandler(m)
	p := SendMessageParams{SessionID: s.ID, RegenerateMessageID: target.ID, Content: "must be ignored", BaseURL: "http://test", ModelName: "model", ProviderType: "openai_compatibility"}
	if err := h.SendMessage(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if r.lastMessage.Content != "original question" {
		t.Fatalf("wrong original question: %s", r.lastMessage.Content)
	}
	if len(r.lastRunOpts.Messages) != 2 || r.lastRunOpts.Messages[1].Content != "previous answer" {
		t.Fatalf("old output leaked into retry history: %+v", r.lastRunOpts.Messages)
	}
	if r.lastRunOpts.Instruction != "fixed instruction" {
		t.Fatalf("fixed prompt changed: %q", r.lastRunOpts.Instruction)
	}
	r.events <- &event.Event{Response: &model.Response{Choices: []model.Choice{{Message: model.NewAssistantMessage("new answer")}}}}
	close(r.events)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		after, _ := m.Storage().ListMessagesForSession(s.ID, 0, 100)
		if len(after) == 4 && after[3].Content == "new answer" {
			if after[2].ID != before[2].ID {
				t.Fatal("question was duplicated")
			}
			if _, err := m.Storage().RegenerationTurn(s.ID, target.ID); err == nil {
				t.Fatal("stale response can be regenerated")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("new answer was not persisted")
}

func TestRegenerationStartFailurePreservesAnswer(t *testing.T) {
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	m := NewManager(newTestStorage(t))
	s, _ := m.Storage().CreateSession(data_models.Session{Title: "retry failure"})
	m.Storage().CreateMessage(data_models.Message{SessionID: s.ID, Role: "user", ContentType: "text", Content: "question"})
	answer, _ := m.Storage().CreateMessage(data_models.Message{SessionID: s.ID, Role: "assistant", ContentType: "text", Content: "keep answer"})
	injectSessionRunner(t, m, s.ID, "http://test", "model", "", &failingRegenerationRunner{newDelayedRunner()})
	err := NewChatHandler(m).SendMessage(context.Background(), SendMessageParams{SessionID: s.ID, RegenerateMessageID: answer.ID, BaseURL: "http://test", ModelName: "model"})
	if err == nil {
		t.Fatal("expected runner failure")
	}
	after, _ := m.Storage().ListMessagesForSession(s.ID, 0, 100)
	if len(after) != 2 || after[1].Content != "keep answer" {
		t.Fatalf("lost original answer: %+v", after)
	}
}
