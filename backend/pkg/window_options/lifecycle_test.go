package window_options

import (
	"testing"

	"github.com/reatang/fire-poker/backend/pkg/id/window_id"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestRootWindowClose(t *testing.T) {
	for _, tc := range []struct {
		name    string
		window  string
		hasHome bool
		quit    bool
	}{
		{"home with hidden quick chat", window_id.Home, true, true},
		{"onboarding with hidden quick chat", window_id.Onboarding, false, true},
		{"onboarding handoff to home", window_id.Onboarding, true, false},
		{"quick chat", window_id.QuickChat, true, false},
		{"settings", window_id.Settings, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			event := &application.WindowEvent{}
			quitCalls := 0
			closeRootWindow(event, tc.window, tc.hasHome, func() { quitCalls++ })
			if (quitCalls == 1) != tc.quit {
				t.Fatalf("quit calls = %d, want quit = %v", quitCalls, tc.quit)
			}
			if event.IsCancelled() != tc.quit {
				t.Fatalf("ordinary close cancelled = %v, want %v", event.IsCancelled(), tc.quit)
			}
		})
	}
}

func TestFileDropRequiresComposerTarget(t *testing.T) {
	files := []string{"/pictures/photo.png"}
	for _, target := range []*application.DropTargetDetails{nil, {}, {Attributes: map[string]string{"id": "other"}}} {
		if fileDropPayload(files, target) != nil {
			t.Fatal("drop outside a composer must not be broadcast")
		}
	}
	target := &application.DropTargetDetails{Attributes: map[string]string{"data-composer-id": "draft-123"}}
	payload := fileDropPayload(files, target)
	if payload["target"] != "draft-123" || payload["files"].([]string)[0] != files[0] {
		t.Fatalf("incorrect drop routing: %#v", payload)
	}
	if fileDropPayload(nil, target) != nil {
		t.Fatal("empty drops must not be broadcast")
	}
}
