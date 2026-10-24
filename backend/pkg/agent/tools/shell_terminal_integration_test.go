package tools

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	pkgterminal "github.com/reatang/fire-poker/backend/pkg/terminal"
	"github.com/reatang/fire-poker/backend/storage"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Exercise the same real shared terminal manager wired by application startup.
func TestShellWithRealTerminalManager(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "terminal.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store, err := storage.NewStorageFromDB(db)
	if err != nil {
		t.Fatal(err)
	}
	manager := pkgterminal.NewManager(store, nil)
	for _, pty := range []bool{false, true} {
		for _, visible := range []bool{false, true} {
			t.Run(fmt.Sprintf("pty_%v_visible_%v", pty, visible), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				command := "pwd; printf 'terminal-ok'"
				if runtime.GOOS == "windows" {
					command = "Get-Location; [Console]::Out.Write('terminal-ok')"
				}
				out, err := shellFuncWithTerminal(ctx, nil, 0, shellInput{Command: command, WorkDir: t.TempDir(), UsePty: pty, ShowTerminal: visible, TimeoutSeconds: 10}, manager)
				if err != nil || out.ExitCode != 0 || !strings.Contains(out.Stdout, "terminal-ok") {
					t.Fatalf("output=%+v err=%v", out, err)
				}
				info, err := manager.WaitTerminal(ctx, out.TerminalID)
				if err != nil || info.Status != "done" || info.ExitCode == nil || *info.ExitCode != 0 {
					t.Fatalf("terminal not fully persisted: %+v err=%v", info, err)
				}
			})
		}
	}
}
