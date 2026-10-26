package file

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/reatang/fire-poker/backend/service/file/file_dto"
)

func TestValidateAttachment(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name  string
		size  int64
		valid bool
	}{
		{"empty.txt", 0, true},
		{"limit.bin", 20 * 1024 * 1024, true},
		{"large.bin", 20*1024*1024 + 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.name)
			file, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := file.Truncate(tc.size); err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			_, err = (&File{}).ValidateAttachment(context.Background(), file_dto.OpenFileInput{Path: path})
			if (err == nil) != tc.valid {
				t.Fatalf("valid = %v, error = %v", tc.valid, err)
			}
		})
	}
	for _, path := range []string{"", " ", dir, filepath.Join(dir, "missing.txt")} {
		if _, err := (&File{}).ValidateAttachment(context.Background(), file_dto.OpenFileInput{Path: path}); err == nil {
			t.Fatalf("expected invalid attachment: %q", path)
		}
	}
}
