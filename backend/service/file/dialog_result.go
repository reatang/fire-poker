package file

import (
	"errors"
	"syscall"
)

// Wails beta.34 forwards cfd.ErrorCancelled from an internal package we cannot
// import. Match that exact sentinel message, including through wrapped errors.
// Other native dialog failures must continue to reach the caller.
func normalizeDialogSelection(path string, err error) (string, error) {
	// Native Windows cancellation may also be exposed directly as ERROR_CANCELLED
	// or HRESULT_FROM_WIN32(ERROR_CANCELLED), independently of the system language.
	var coded interface{ Code() uintptr }
	if errors.Is(err, syscall.Errno(1223)) || (errors.As(err, &coded) && uint32(coded.Code()) == 0x800704c7) {
		return "", nil
	}
	for cause := err; cause != nil; cause = errors.Unwrap(cause) {
		if cause.Error() == "cancelled by user" {
			return "", nil
		}
	}
	return path, err
}
