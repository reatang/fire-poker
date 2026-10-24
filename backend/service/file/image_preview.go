package file

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/reatang/fire-poker/backend/service/file/file_dto"
)

// ImagePreview reads supported raster images for display inside the webview.
func (f *File) ImagePreview(ctx context.Context, input file_dto.ImagePreviewInput) (*file_dto.ImagePreviewOutput, error) {
	file, err := os.Open(input.Path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	const maxBytes = 20 * 1024 * 1024
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxBytes {
		return nil, fmt.Errorf("image preview requires a regular file up to 20 MB")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBytes {
		return nil, fmt.Errorf("image preview exceeds 20 MB")
	}
	mime := http.DetectContentType(data)
	switch mime {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
	default:
		return nil, fmt.Errorf("unsupported image preview type: %s", mime)
	}
	return &file_dto.ImagePreviewOutput{DataURL: "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)}, nil
}
