package agent

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/reatang/fire-poker/backend/models/data_models"
	"github.com/reatang/fire-poker/backend/pkg/agent/toolkit"
	"github.com/reatang/fire-poker/backend/service/agent/agent_dto"
	"github.com/reatang/fire-poker/backend/service/settings"
	"github.com/reatang/fire-poker/backend/service/settings/settings_dto"
	"github.com/reatang/fire-poker/backend/storage"
)

// TestSessionToolsAreIndependentSnapshots verifies persistence across settings changes and service recreation.
func TestSessionToolsAreIndependentSnapshots(t *testing.T) {
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	store := newTestStorage(t)
	config := settings.NewSettings(settings.Dependencies{SessionToolDefaults: store})
	ctx := context.Background()
	if _, err := config.SaveToolSettings(ctx, settings_dto.SaveToolSettingsInput{DefaultToolIDs: []string{"web_fetch"}}); err != nil {
		t.Fatal(err)
	}
	a := NewAgent(store)
	first, err := a.CreateSession(ctx, agent_dto.CreateSessionInput{})
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.CreateSession(ctx, agent_dto.CreateSessionInput{})
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := store.CreateSession(data_models.Session{Title: "legacy"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.SaveSessionToolSettings(ctx, agent_dto.SaveSessionToolsInput{SessionID: first.SessionID, ToolIDs: []string{}}); err != nil {
		t.Fatal(err)
	}
	if _, err := config.SaveToolSettings(ctx, settings_dto.SaveToolSettingsInput{DefaultToolIDs: []string{"shell"}}); err != nil {
		t.Fatal(err)
	}
	third, err := a.CreateSession(ctx, agent_dto.CreateSessionInput{})
	if err != nil {
		t.Fatal(err)
	}
	reopened := NewAgent(store)
	for _, tc := range []struct {
		id   uint
		want []string
	}{
		{first.SessionID, []string{}}, {second.SessionID, []string{"web_fetch"}}, {legacy.ID, []string{"web_fetch"}}, {third.SessionID, []string{"shell"}},
	} {
		got, err := reopened.GetSessionToolSettings(ctx, agent_dto.SessionToolsInput{SessionID: tc.id})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got.ToolIDs, toolkit.NormalizeToolIDs(tc.want)) {
			t.Fatalf("session %d: got %v want %v", tc.id, got.ToolIDs, tc.want)
		}
	}
}

// TestStartedSessionToolsCannotChange covers direct saves, continuation overrides and archived history.
func TestStartedSessionToolsCannotChange(t *testing.T) {
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	store := newTestStorage(t)
	a := NewAgent(store)
	ctx := context.Background()
	session, err := a.CreateSession(ctx, agent_dto.CreateSessionInput{})
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{"shell", "web_fetch"}
	if _, err := a.SaveSessionToolSettings(ctx, agent_dto.SaveSessionToolsInput{SessionID: session.SessionID, ToolIDs: ids}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateMessage(data_models.Message{SessionID: session.SessionID, Role: "user", ContentType: "text", Content: "started"}); err != nil {
		t.Fatal(err)
	}
	got, err := a.GetSessionToolSettings(ctx, agent_dto.SessionToolsInput{SessionID: session.SessionID})
	if err != nil || !got.Locked {
		t.Fatalf("started session was not locked: %v", err)
	}
	if _, err := a.SaveSessionToolSettings(ctx, agent_dto.SaveSessionToolsInput{SessionID: session.SessionID, ToolIDs: []string{}}); err == nil {
		t.Fatal("API changed locked tools")
	}
	if _, err := a.manager.ToolsForTurn(session.SessionID, []string{}); !errors.Is(err, storage.ErrSessionToolsLocked) {
		t.Fatalf("turn bypassed tool lock: %v", err)
	}
	for _, request := range [][]string{nil, {"web_fetch", "shell"}} {
		resolved, err := a.manager.ToolsForTurn(session.SessionID, request)
		if err != nil || !reflect.DeepEqual(resolved, toolkit.NormalizeToolIDs(ids)) {
			t.Fatalf("unchanged continuation rejected: %v", err)
		}
	}
	if err := store.DeleteMessagesForSession(session.SessionID); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateSessionTools(session.SessionID, []string{}); !errors.Is(err, storage.ErrSessionToolsLocked) {
		t.Fatal("archiving history unlocked tools")
	}
	if _, err := a.manager.ToolsForTurn(session.SessionID, []string{}); !errors.Is(err, storage.ErrSessionToolsLocked) {
		t.Fatal("archived conversation accepted override")
	}
}
