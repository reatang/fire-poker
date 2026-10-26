package agent

import (
	"os"
	"path/filepath"
	"testing"
)

// TestStoreSessionAttachmentsKeepsNamesWithoutOverwriting copies files under their display names,
// suffixes clashes, and leaves files already in the session directory where they are.
func TestStoreSessionAttachmentsKeepsNamesWithoutOverwriting(t *testing.T) {
	src := t.TempDir()
	first := filepath.Join(src, "123-image.png")
	second := filepath.Join(src, "456-image.png")
	for path, body := range map[string]string{first: "one", second: "two"} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	workDir := t.TempDir()

	stored, err := storeSessionAttachments(workDir, []Attachment{
		{Name: "image.png", Path: first, Kind: "image"},
		{Name: "image.png", Path: second, Kind: "image"},
	})
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	target := filepath.Join(workDir, sessionAttachmentsDir)
	want := []struct{ path, body string }{
		{filepath.Join(target, "image.png"), "one"},
		{filepath.Join(target, "image-1.png"), "two"},
	}
	for i, w := range want {
		if stored[i].Path != w.path {
			t.Fatalf("attachment %d stored at %s, want %s", i, stored[i].Path, w.path)
		}
		if data, err := os.ReadFile(w.path); err != nil || string(data) != w.body {
			t.Fatalf("attachment %d content = %q, %v", i, data, err)
		}
	}

	again, err := storeSessionAttachments(workDir, stored[:1])
	if err != nil {
		t.Fatalf("store again: %v", err)
	}
	if again[0].Path != stored[0].Path {
		t.Fatalf("already stored attachment was copied again: %s", again[0].Path)
	}
}

// TestRemoveSessionWorkDirDeletesAttachments ties the directory's lifetime to the session.
func TestRemoveSessionWorkDirDeletesAttachments(t *testing.T) {
	manager := NewManager(newTestStorage(t))
	workDir := manager.SessionWorkDir(42)
	if workDir == "" {
		t.Fatal("expected a working directory")
	}
	if _, err := storeSessionAttachments(workDir, nil); err != nil {
		t.Fatalf("store: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "note.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := manager.RemoveSessionWorkDir(42); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(workDir); !os.IsNotExist(err) {
		t.Fatalf("working directory still exists: %v", err)
	}
	if manager.SessionWorkDir(0) != "" {
		t.Fatal("unsaved sessions must not get a working directory")
	}
}
