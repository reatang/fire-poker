package file

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/reatang/fire-poker/backend/pkg/ierror"
	"github.com/reatang/fire-poker/backend/service/file/file_dto"
)

// ValidateAttachment checks native file paths before they enter a composer draft.
func (f *File) ValidateAttachment(ctx context.Context, input file_dto.OpenFileInput) (*file_dto.OpenFileOutput, error) {
	path := strings.TrimSpace(input.Path)
	if path == "" {
		return nil, ierror.Error(ierror.ErrAgentAttachmentPath, fmt.Errorf("attachment path is empty"))
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, ierror.Error(ierror.ErrAgentAttachmentNotFound, err)
	}
	if !info.Mode().IsRegular() {
		return nil, ierror.Error(ierror.ErrAgentAttachmentPath, fmt.Errorf("attachment must be a regular file: %s", path))
	}
	if info.Size() > 20*1024*1024 {
		return nil, ierror.Error(ierror.ErrAgentAttachmentSize, fmt.Errorf("attachment %q exceeds 20 MB", info.Name()))
	}
	return &file_dto.OpenFileOutput{}, nil
}
