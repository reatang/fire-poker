package main

import (
	"testing"

	"github.com/reatang/fire-poker/backend/pkg/window_options"
)

func TestDefaultHomeEnableFileDrop(t *testing.T) {
	options := window_options.DefaultHome()

	if !options.EnableFileDrop {
		t.Fatal("expected file drop to be enabled for the main window")
	}
}

func TestQuickChatShortcutHasOneOwner(t *testing.T) {
	for _, goos := range []string{"windows", "darwin"} {
		if bindings := quickChatFallbackBindings(goos, "Ctrl+Shift+Space", func() {}); len(bindings) != 0 {
			t.Fatalf("%s must use only the global hotkey registration", goos)
		}
	}
	calls := 0
	bindings := quickChatFallbackBindings("linux", "Ctrl+Shift+Space", func() { calls++ })
	bindings["Ctrl+Shift+Space"](nil)
	if calls != 1 {
		t.Fatalf("fallback calls = %d, want 1", calls)
	}
}
