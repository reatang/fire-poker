package agent

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/reatang/fire-poker/backend/pkg/dir"
)

// sessionAttachmentsDir is the subdirectory of a session's working directory that holds its attachments.
const sessionAttachmentsDir = "attachments"

// StoreSessionAttachments gives uploaded files the same lifetime as their session,
// before they are used for a preview or a message.
func (m *Manager) StoreSessionAttachments(sessionID uint, atts []Attachment) ([]Attachment, error) {
	if sessionID == 0 {
		return nil, fmt.Errorf("attachments require a saved session")
	}
	if _, err := m.Storage().GetSession(sessionID); err != nil {
		return nil, fmt.Errorf("load attachment session: %w", err)
	}
	workDir := m.SessionWorkDir(sessionID)
	if workDir == "" {
		return nil, fmt.Errorf("session %d has no attachment directory", sessionID)
	}
	return storeSessionAttachments(workDir, atts)
}

// SessionWorkDir returns the conversation's working directory, creating it on first use.
// It returns "" for an unsaved session or when the directory cannot be created.
func (m *Manager) SessionWorkDir(sessionID uint) string {
	if sessionID == 0 {
		return ""
	}
	path := dir.SessionDir(mustDataDir(), sessionID)
	if err := os.MkdirAll(path, 0o755); err != nil {
		log.Printf("create session %d work dir: %v", sessionID, err)
		return ""
	}
	return path
}

// RemoveSessionWorkDir deletes the conversation's working directory together with its attachments.
func (m *Manager) RemoveSessionWorkDir(sessionID uint) error {
	if sessionID == 0 {
		return nil
	}
	return os.RemoveAll(dir.SessionDir(mustDataDir(), sessionID))
}

// storeSessionAttachments copies attachments into {workDir}/attachments so a question keeps its
// files after the temp directory is cleared or the originals move. Files already there are kept as is.
func storeSessionAttachments(workDir string, atts []Attachment) ([]Attachment, error) {
	if len(atts) == 0 {
		return atts, nil
	}
	target := filepath.Join(workDir, sessionAttachmentsDir)
	if err := os.MkdirAll(target, 0o755); err != nil {
		return nil, fmt.Errorf("create attachments dir: %w", err)
	}
	out := make([]Attachment, 0, len(atts))
	for _, a := range atts {
		if filepath.Dir(filepath.Clean(a.Path)) == target {
			out = append(out, a)
			continue
		}
		name := filepath.Base(a.Name)
		if name == "" || name == "." || name == string(filepath.Separator) {
			name = filepath.Base(a.Path)
		}
		stored, err := copyIntoDir(a.Path, target, name)
		if err != nil {
			return nil, fmt.Errorf("store attachment %q: %w", name, err)
		}
		a.Path = stored
		out = append(out, a)
	}
	return out, nil
}

// copyIntoDir copies src to dir/name, adding a numeric suffix rather than overwriting an existing file.
func copyIntoDir(src, dir, name string) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()

	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 0; ; i++ {
		candidate := name
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d%s", stem, i, ext)
		}
		path := filepath.Join(dir, candidate)
		out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		_, copyErr := io.Copy(out, in)
		if err := errors.Join(copyErr, out.Close()); err != nil {
			_ = os.Remove(path)
			return "", err
		}
		return path, nil
	}
}
