package storage

import (
	"fmt"
	"github.com/reatang/fire-poker/backend/models/data_models"
	"testing"
)

func TestForkCopiesCompletePrefixWithIndependentIDsAndPrompt(t *testing.T) {
	s := newTestStorage(t)
	source, _ := s.CreateSession(data_models.Session{Title: "original", SystemPrompt: "fixed role", PromptName: "writer", Kind: "task", Tags: `["task"]`, Starred: true})
	question, _ := s.CreateMessage(data_models.Message{SessionID: source.ID, Role: "user", ContentType: "text", Content: "question", Attachments: `[{"path":"D:/test/a.txt","name":"a.txt"}]`})
	parent := question.ID
	for i := 0; i < 120; i++ {
		if _, err := s.CreateMessage(data_models.Message{SessionID: source.ID, ParentID: &parent, Role: "assistant", ContentType: "thinking", Content: fmt.Sprintf("thinking %d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	target, _ := s.CreateMessage(data_models.Message{SessionID: source.ID, ParentID: &parent, Role: "assistant", ContentType: "text", Content: "boundary answer", ModelName: "m", TokensIn: 42, Extra: "interrupted"})
	s.CreateMessage(data_models.Message{SessionID: source.ID, Role: "user", ContentType: "text", Content: "must not copy"})
	fork, err := s.ForkSession(source.ID, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fork.ID == source.ID || fork.Title != "original · 分叉" || fork.Status != "idle" || fork.Kind != "user" || fork.Starred {
		t.Fatalf("bad fork: %+v", fork)
	}
	if fork.SystemPrompt != source.SystemPrompt || fork.PromptName != source.PromptName {
		t.Fatal("prompt not inherited")
	}
	copied, err := s.ListMessagesForSession(fork.ID, 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	if len(copied) != 122 || copied[121].Content != "boundary answer" {
		t.Fatalf("wrong prefix length %d", len(copied))
	}
	if copied[0].ID == question.ID || copied[0].Attachments != question.Attachments || !copied[0].CreatedAt.Equal(question.CreatedAt) {
		t.Fatal("original question/attachments were not cloned")
	}
	for _, m := range copied[1:] {
		if m.SessionID != fork.ID || m.ParentID == nil || *m.ParentID != copied[0].ID {
			t.Fatal("parent still points to source")
		}
	}
	if copied[121].TokensIn != 42 || copied[121].Extra != "interrupted" {
		t.Fatal("reply metadata was lost")
	}
	original, _ := s.ListMessagesForSession(source.ID, 0, -1)
	if len(original) != 123 || original[122].Content != "must not copy" {
		t.Fatal("source changed")
	}
	if err := s.UpdateSessionPrompt(fork.ID, "different", "changed"); err == nil {
		t.Fatal("started fork prompt was editable")
	}
}

func TestForkRejectsForeignOrUnsupportedBoundaryWithoutCreatingSession(t *testing.T) {
	s := newTestStorage(t)
	a, _ := s.CreateSession(data_models.Session{Title: "a"})
	b, _ := s.CreateSession(data_models.Session{Title: "b"})
	text, _ := s.CreateMessage(data_models.Message{SessionID: a.ID, Role: "user", ContentType: "text", Content: "q"})
	thinking, _ := s.CreateMessage(data_models.Message{SessionID: a.ID, Role: "assistant", ContentType: "thinking", Content: "reason"})
	for _, ids := range [][2]uint{{b.ID, text.ID}, {a.ID, thinking.ID}, {a.ID, 99999}} {
		if _, err := s.ForkSession(ids[0], ids[1]); err == nil {
			t.Fatal("invalid boundary accepted")
		}
	}
	sessions, _ := s.ListSessions(0, 100, false, true)
	if len(sessions) != 2 {
		t.Fatal("failed fork left a session behind")
	}
}

func TestForkRollsBackWhenCopyingMessagesFails(t *testing.T) {
	s := newTestStorage(t)
	source, _ := s.CreateSession(data_models.Session{Title: "source"})
	target, _ := s.CreateMessage(data_models.Message{SessionID: source.ID, Role: "user", ContentType: "text", Content: "q"})
	if err := s.sqliteDB.Exec(fmt.Sprintf("CREATE TRIGGER reject_fork BEFORE INSERT ON messages WHEN NEW.session_id <> %d BEGIN SELECT RAISE(ABORT, 'test failure'); END", source.ID)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.ForkSession(source.ID, target.ID); err == nil {
		t.Fatal("expected copy failure")
	}
	sessions, _ := s.ListSessions(0, 100, false, true)
	if len(sessions) != 1 {
		t.Fatal("fork session was not rolled back")
	}
	messages, _ := s.ListMessagesForSession(source.ID, 0, -1)
	if len(messages) != 1 {
		t.Fatal("source modified")
	}
}
