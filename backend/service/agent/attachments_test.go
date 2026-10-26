package agent

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/reatang/fire-poker/backend/pkg/dir"
	"github.com/reatang/fire-poker/backend/service/agent/agent_dto"
	fileSvc "github.com/reatang/fire-poker/backend/service/file"
	"github.com/reatang/fire-poker/backend/service/file/file_dto"
)

func TestStoreAttachmentsPersistsBeforeSendingAndSurvivesTempRemoval(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("FIRE_POKER_DATA_DIR", dataDir)
	svc := NewAgent(newTestStorage(t))
	ctx := context.Background()
	session, err := svc.CreateSession(ctx, agent_dto.CreateSessionInput{})
	if err != nil {
		t.Fatal(err)
	}
	png, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aN1sAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "temporary-image.png")
	if err := os.WriteFile(source, png, 0o600); err != nil {
		t.Fatal(err)
	}
	stored, err := svc.StoreAttachments(ctx, agent_dto.StoreAttachmentsInput{
		SessionID: session.SessionID, Attachments: []agent_dto.AttachmentInput{{Path: source, Name: "photo.png", Mime: "image/png"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Attachments) != 1 {
		t.Fatalf("stored attachments: %+v", stored)
	}
	path := stored.Attachments[0].Path
	want := filepath.Join(dir.SessionDir(dataDir, session.SessionID), "attachments", "photo.png")
	if path != want {
		t.Fatalf("stored at %s, want %s", path, want)
	}
	if err := os.Remove(source); err != nil {
		t.Fatal(err)
	}
	preview, err := (&fileSvc.File{}).ImagePreview(ctx, file_dto.ImagePreviewInput{Path: path})
	if err != nil || preview == nil || preview.DataURL == "" {
		t.Fatalf("preview after temp removal: %+v, %v", preview, err)
	}
	messages, err := svc.LoadSessionMessages(ctx, agent_dto.LoadSessionMessagesInput{SessionID: session.SessionID})
	if err != nil || len(messages.Messages) != 0 {
		t.Fatalf("upload must not send a message: %+v, %v", messages, err)
	}
	// Sending an already staged attachment must not create another copy.
	again, err := svc.StoreAttachments(ctx, agent_dto.StoreAttachmentsInput{
		SessionID: session.SessionID, Attachments: []agent_dto.AttachmentInput{{Path: path}},
	})
	if err != nil || again.Attachments[0].Path != path {
		t.Fatalf("staged path changed: %+v, %v", again, err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Fatalf("upload created duplicate copies: %v, %v", entries, err)
	}
	// Another session owns its own copy, so deleting the source session is safe.
	other, err := svc.CreateSession(ctx, agent_dto.CreateSessionInput{})
	if err != nil {
		t.Fatal(err)
	}
	otherCopy, err := svc.StoreAttachments(ctx, agent_dto.StoreAttachmentsInput{
		SessionID: other.SessionID, Attachments: []agent_dto.AttachmentInput{{Path: path}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.manager.RemoveSessionWorkDir(session.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(otherCopy.Attachments[0].Path); err != nil {
		t.Fatalf("other session lost its attachment: %v", err)
	}
}

func TestStoreAttachmentsRejectsUnknownSession(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("FIRE_POKER_DATA_DIR", dataDir)
	svc := NewAgent(newTestStorage(t))
	source := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(source, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, id := range []uint{0, 999} {
		_, err := svc.StoreAttachments(context.Background(), agent_dto.StoreAttachmentsInput{
			SessionID: id, Attachments: []agent_dto.AttachmentInput{{Path: source}},
		})
		if err == nil {
			t.Fatalf("accepted unknown session %d", id)
		}
		if _, err := os.Stat(dir.SessionDir(dataDir, id)); !os.IsNotExist(err) {
			t.Fatalf("created directory for unknown session %d", id)
		}
	}
}
