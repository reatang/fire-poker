package agent

import (
	"context"
	"testing"
)

// TestSettingsWritesCannotOverlapReservedTurn protects the interval before the first message is saved.
func TestSettingsWritesCannotOverlapReservedTurn(t *testing.T) {
	sm := NewStreamManager()
	_, cancel, err := sm.TryStart(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	called := false
	if err := sm.WhileInactive(7, func() error { called = true; return nil }); err == nil || called {
		t.Fatal("settings write overlapped reserved turn")
	}
	sm.Remove(7)
	if err := sm.WhileInactive(7, func() error { called = true; return nil }); err != nil || !called {
		t.Fatal("inactive session could not save settings")
	}
}
