package file

import (
	"context"
	"os"
	"path/filepath"

	pkgdir "github.com/reatang/fire-poker/backend/pkg/dir"
	"github.com/wailsapp/wails/v3/pkg/application"
)

func (f *File) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	f.wailsApp = application.Get()
	cleanTempDir(pkgdir.TempRoot())
	return nil
}

// cleanTempDir removes all regular files from the given directory.
// Called at startup to prevent unbounded disk growth across sessions.
func cleanTempDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
