package storage

import (
	"fmt"
	"sync"
	"testing"

	"github.com/reatang/fire-poker/backend/models/data_models"
)

// newFileTestStorage opens the real on-disk database the app uses, inside a throwaway data dir.
func newFileTestStorage(t *testing.T) *Storage {
	t.Helper()
	t.Setenv("FIRE_POKER_DATA_DIR", t.TempDir())
	stor, err := NewStorage()
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := stor.sqliteDB.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return stor
}

// TestNewStorageConfiguresConcurrentAccess pins the pragmas that let background writers share the database.
func TestNewStorageConfiguresConcurrentAccess(t *testing.T) {
	stor := newFileTestStorage(t)

	var journalMode string
	if err := stor.sqliteDB.Raw("PRAGMA journal_mode").Scan(&journalMode).Error; err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("expected WAL journal mode, got %q", journalMode)
	}
	var busyTimeout int
	if err := stor.sqliteDB.Raw("PRAGMA busy_timeout").Scan(&busyTimeout).Error; err != nil {
		t.Fatalf("read busy_timeout: %v", err)
	}
	if busyTimeout < 1000 {
		t.Fatalf("expected a busy timeout of at least 1s, got %dms", busyTimeout)
	}
}

// TestNewStorageHandlesConcurrentWriters mirrors streaming replies, terminal output and memory
// encoding writing at once; read-then-write transactions must wait for the lock instead of failing.
func TestNewStorageHandlesConcurrentWriters(t *testing.T) {
	stor := newFileTestStorage(t)
	session, err := stor.CreateSession(data_models.Session{Title: "concurrent"})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	terminalIDs := []string{"term-a", "term-b"}
	for _, id := range terminalIDs {
		if _, err := stor.CreateTerminal(data_models.Terminal{TerminalID: id, SessionID: session.ID, Status: "active"}); err != nil {
			t.Fatalf("create terminal %s: %v", id, err)
		}
	}

	const workers = 8
	const opsPerWorker = 25
	var wg sync.WaitGroup
	errs := make(chan error, workers*opsPerWorker)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < opsPerWorker; i++ {
				var err error
				switch i % 3 {
				case 0:
					_, err = stor.CreateMessage(data_models.Message{SessionID: session.ID, Role: "assistant", ContentType: "text", Content: fmt.Sprintf("w%d-%d", w, i)})
				case 1:
					_, err = stor.AppendTerminalOutput(terminalIDs[w%len(terminalIDs)], "x")
				default:
					_, err = stor.CreateMemory(data_models.Memory{Summary: fmt.Sprintf("m%d-%d", w, i), Content: "c", Type: "fact", Target: "user"})
				}
				if err != nil {
					errs <- err
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent write failed: %v", err)
	}

	// Each terminal's chunks must form one gapless sequence with matching byte cursors.
	for _, id := range terminalIDs {
		chunks, err := stor.ReadTerminalOutput(id, 0)
		if err != nil {
			t.Fatalf("read terminal %s: %v", id, err)
		}
		for i, chunk := range chunks {
			if chunk.Seq != int64(i+1) || chunk.CursorStart != int64(i) || chunk.CursorEnd != int64(i+1) {
				t.Fatalf("terminal %s chunk %d out of order: %+v", id, i, chunk)
			}
		}
	}
}
