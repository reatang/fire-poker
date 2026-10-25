package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestLoggerWritesOnlyMessagesAtOrAboveConfiguredLevel verifies level filtering and daily log file creation.
func TestLoggerWritesOnlyMessagesAtOrAboveConfiguredLevel(t *testing.T) {
	tempDir := t.TempDir()
	clock := func() time.Time {
		return time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	}

	lg, err := New(Options{
		DataDir:  tempDir,
		Level:    "warn",
		Now:      clock,
		FileMode: 0o644,
		DirMode:  0o755,
	})
	if err != nil {
		t.Fatal(err)
	}

	lg.Info("skip this")
	lg.Warn("keep this")
	lg.Error("keep this too")

	bytes, err := os.ReadFile(filepath.Join(tempDir, "logs", "2026-05-10.log"))
	if err != nil {
		t.Fatal(err)
	}

	logText := string(bytes)
	if strings.Contains(logText, "skip this") {
		t.Fatalf("info message should not have been written: %s", logText)
	}
	if !strings.Contains(logText, "keep this") || !strings.Contains(logText, "keep this too") {
		t.Fatalf("expected warn and error messages in log file: %s", logText)
	}
}

// TestLoggerWriteMirrorsStandardLogLines verifies io.Writer output lands verbatim in the daily file.
func TestLoggerWriteMirrorsStandardLogLines(t *testing.T) {
	tempDir := t.TempDir()
	lg, err := New(Options{DataDir: tempDir, Level: "error", Now: func() time.Time {
		return time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	}})
	if err != nil {
		t.Fatal(err)
	}

	n, err := lg.Write([]byte("2026/05/10 12:00:00 startup ok\n"))
	if err != nil || n != len("2026/05/10 12:00:00 startup ok\n") {
		t.Fatalf("write returned %d, %v", n, err)
	}

	bytes, err := os.ReadFile(filepath.Join(tempDir, "logs", "2026-05-10.log"))
	if err != nil {
		t.Fatal(err)
	}
	if string(bytes) != "2026/05/10 12:00:00 startup ok\n" {
		t.Fatalf("raw lines must be written verbatim regardless of level, got %q", string(bytes))
	}
}

// TestLoggerPruneRemovesOnlyExpiredDailyFiles keeps recent days and unrelated files.
func TestLoggerPruneRemovesOnlyExpiredDailyFiles(t *testing.T) {
	tempDir := t.TempDir()
	logDir := filepath.Join(tempDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := []string{"2026-04-25.log", "2026-04-26.log", "2026-05-10.log", "notes.txt"}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(logDir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	lg, err := New(Options{DataDir: tempDir, Now: func() time.Time {
		return time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := lg.Prune(14); err != nil {
		t.Fatal(err)
	}

	for name, wantExists := range map[string]bool{"2026-04-25.log": false, "2026-04-26.log": true, "2026-05-10.log": true, "notes.txt": true} {
		_, err := os.Stat(filepath.Join(logDir, name))
		if exists := err == nil; exists != wantExists {
			t.Fatalf("%s exists = %v, want %v", name, exists, wantExists)
		}
	}
}

// TestLoggerPruneWithoutLogDir is a no-op on a fresh install.
func TestLoggerPruneWithoutLogDir(t *testing.T) {
	lg, err := New(Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := lg.Prune(14); err != nil {
		t.Fatalf("prune on missing dir: %v", err)
	}
}
