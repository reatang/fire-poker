package agent

import (
	"context"
	"github.com/reatang/fire-poker/backend/models/data_models"
	"github.com/reatang/fire-poker/backend/service/agent/agent_dto"
	"testing"
)

func TestSessionPromptPersistenceLibraryAndReset(t *testing.T) {
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	stor := newTestStorage(t)
	a := NewAgent(stor)
	ctx := context.Background()
	s, err := a.CreateSession(ctx, agent_dto.CreateSessionInput{})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := a.UpdateSessionPrompt(ctx, agent_dto.UpdateSessionPromptInput{SessionID: s.SessionID, System: "你是企业架构顾问", Name: "架构顾问"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.IsDefault {
		t.Fatal("custom prompt treated as default")
	}
	// Reconstruct service to ensure the value is persisted rather than held in UI/manager memory.
	b := NewAgent(stor)
	got, err := b.GetSessionPrompt(ctx, agent_dto.SessionPromptInput{SessionID: s.SessionID})
	if err != nil || got.System != updated.System {
		t.Fatalf("prompt did not persist: %+v %v", got, err)
	}
	if _, err = b.SaveSystemPrompt(ctx, agent_dto.PromptLibraryInput{Name: "架构顾问", System: got.System}); err != nil {
		t.Fatal(err)
	}
	list, err := b.ListSystemPrompts(ctx)
	if err != nil || len(list.Names) != 1 {
		t.Fatalf("library missing prompt: %+v %v", list, err)
	}
	if _, err = b.SaveSystemPrompt(ctx, agent_dto.PromptLibraryInput{Name: "../escape", System: "invalid"}); err == nil {
		t.Fatal("path traversal accepted")
	}
	reset, err := b.UpdateSessionPrompt(ctx, agent_dto.UpdateSessionPromptInput{SessionID: s.SessionID, Reset: true})
	if err != nil || !reset.IsDefault || reset.System != reset.DefaultSystem {
		t.Fatalf("reset failed: %+v %v", reset, err)
	}
	library, err := b.GetSystemPrompt(ctx, agent_dto.PromptLibraryInput{Name: "架构顾问"})
	if err != nil || library.System != updated.System {
		t.Fatalf("reset modified library: %+v %v", library, err)
	}
}

func TestStartedSessionCannotChangeOrResetPrompt(t *testing.T) {
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	stor := newTestStorage(t)
	a := NewAgent(stor)
	ctx := context.Background()
	s, err := a.CreateSession(ctx, agent_dto.CreateSessionInput{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.UpdateSessionPrompt(ctx, agent_dto.UpdateSessionPromptInput{SessionID: s.SessionID, System: "固定角色", Name: "架构顾问"}); err != nil {
		t.Fatal(err)
	}
	if _, err = stor.CreateMessage(data_models.Message{SessionID: s.SessionID, Role: "user", ContentType: "text", Content: "你好"}); err != nil {
		t.Fatal(err)
	}
	for _, change := range []agent_dto.UpdateSessionPromptInput{{SessionID: s.SessionID, System: "其他角色", Name: "写作助手"}, {SessionID: s.SessionID, Reset: true}} {
		if _, err = a.UpdateSessionPrompt(ctx, change); err == nil {
			t.Fatal("started conversation accepted a prompt change")
		}
	}
	// Storage enforces the rule atomically, even if callers bypass the service.
	if err = stor.UpdateSessionPrompt(s.SessionID, "绕过接口", "invalid"); err == nil {
		t.Fatal("storage allowed changing a started session")
	}
	got, err := a.GetSessionPrompt(ctx, agent_dto.SessionPromptInput{SessionID: s.SessionID})
	if err != nil || !got.Locked || got.System != "固定角色" || got.Name != "架构顾问" {
		t.Fatalf("locked prompt changed: %+v %v", got, err)
	}
}

func TestDeleteLibraryPromptPreservesSession(t *testing.T) {
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	stor := newTestStorage(t)
	a := NewAgent(stor)
	ctx := context.Background()
	input := agent_dto.PromptLibraryInput{Name: "assistant", System: "Original instructions"}
	if _, err := a.SaveSystemPrompt(ctx, input); err != nil {
		t.Fatal(err)
	}
	s, err := a.CreateSession(ctx, agent_dto.CreateSessionInput{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.UpdateSessionPrompt(ctx, agent_dto.UpdateSessionPromptInput{SessionID: s.SessionID, Name: input.Name, System: input.System}); err != nil {
		t.Fatal(err)
	}
	if _, err := stor.CreateMessage(data_models.Message{SessionID: s.SessionID, Role: "user", ContentType: "text", Content: "Hello"}); err != nil {
		t.Fatal(err)
	}
	if err := a.DeleteSystemPrompt(ctx, input); err != nil {
		t.Fatal(err)
	}
	list, err := a.ListSystemPrompts(ctx)
	if err != nil || len(list.Names) != 0 {
		t.Fatalf("deleted prompt still listed: %+v, %v", list, err)
	}
	got, err := a.GetSessionPrompt(ctx, agent_dto.SessionPromptInput{SessionID: s.SessionID})
	if err != nil || got.System != input.System || got.Name != input.Name || !got.Locked {
		t.Fatalf("deleting a library prompt changed a session: %+v, %v", got, err)
	}
	if err := a.DeleteSystemPrompt(ctx, agent_dto.PromptLibraryInput{Name: "../escape"}); err == nil {
		t.Fatal("path traversal accepted")
	}
	if err := a.DeleteSystemPrompt(ctx, input); err == nil {
		t.Fatal("deleting a missing prompt should fail")
	}
}
