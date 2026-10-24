package file

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/reatang/fire-poker/backend/service/file/file_dto"
)

func TestImagePreview(t *testing.T) {
	image, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aN1sAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	f := &File{}
	for _, tc := range []struct {
		name string
		data []byte
		ok   bool
	}{
		{"image.png", image, true},
		{"fake.png", []byte("not an image"), false},
		{"vector.svg", []byte("<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.name)
			if err := os.WriteFile(path, tc.data, 0o600); err != nil {
				t.Fatal(err)
			}
			out, err := f.ImagePreview(context.Background(), file_dto.ImagePreviewInput{Path: path})
			if tc.ok {
				if err != nil {
					t.Fatal(err)
				}
				if out.DataURL != "data:image/png;base64,"+base64.StdEncoding.EncodeToString(image) {
					t.Fatal("incorrect preview")
				}
			} else if err == nil {
				t.Fatal("expected unsupported image error")
			}
		})
	}
	for _, path := range []string{dir, filepath.Join(dir, "missing.png")} {
		if _, err := f.ImagePreview(context.Background(), file_dto.ImagePreviewInput{Path: path}); err == nil {
			t.Fatal("expected invalid path error")
		}
	}
	path := filepath.Join(dir, "large.png")
	large, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := large.Truncate(20*1024*1024 + 1); err != nil {
		t.Fatal(err)
	}
	large.Close()
	if _, err := f.ImagePreview(context.Background(), file_dto.ImagePreviewInput{Path: path}); err == nil {
		t.Fatal("expected size limit error")
	}
}
