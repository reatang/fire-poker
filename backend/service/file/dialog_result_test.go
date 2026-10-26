package file

import (
	"errors"
	"fmt"
	"syscall"
	"testing"
)

func TestNormalizeDialogSelection(t *testing.T) {
	cancelled := errors.New("cancelled by user")
	failure := errors.New("native dialog failed")
	mention := errors.New("cancelled by user: failed to release dialog")
	denied := dialogTestError(0x80070005)
	for _, tc := range []struct {
		name     string
		path     string
		err      error
		wantPath string
		wantErr  error
	}{
		{"selected file", "C:/photos/image.png", nil, "C:/photos/image.png", nil},
		{"selected folder", "C:/photos", nil, "C:/photos", nil},
		{"empty selection", "", nil, "", nil},
		{"Windows cancel", "", cancelled, "", nil},
		{"wrapped Windows cancel", "", fmt.Errorf("show dialog: %w", cancelled), "", nil},
		{"Windows cancellation code", "", syscall.Errno(1223), "", nil},
		{"Windows COM cancellation", "", dialogTestError(0x800704c7), "", nil},
		{"wrapped COM cancellation", "", fmt.Errorf("show dialog: %w", dialogTestError(0x800704c7)), "", nil},
		{"cancel ignores stale selection", "C:/photos/image.png", cancelled, "", nil},
		{"native failure", "", failure, "", failure},
		{"failure mentioning cancellation", "", mention, "", mention},
		{"Windows COM failure", "", denied, "", denied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, err := normalizeDialogSelection(tc.path, tc.err)
			if path != tc.wantPath {
				t.Fatalf("path = %q, want %q", path, tc.wantPath)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

type dialogTestError uintptr

func (err dialogTestError) Code() uintptr { return uintptr(err) }
func (err dialogTestError) Error() string { return "native dialog error" }
