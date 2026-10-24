package configfile

import (
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"time"
)

// lockFile excludes concurrent writers from other app instances sharing the data directory.
func lockFile(file *os.File) error {
	return windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &windows.Overlapped{})
}

// replaceFile tolerates short-lived Windows read handles while keeping replacement atomic.
func replaceFile(source, target string) error {
	for attempt := 0; ; attempt++ {
		err := os.Rename(source, target)
		if err == nil || attempt >= 25 || (!errors.Is(err, windows.ERROR_SHARING_VIOLATION) && !errors.Is(err, windows.ERROR_ACCESS_DENIED)) {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// readFile retries a transient sharing conflict while another instance replaces config.json.
func readFile(path string) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		data, err := os.ReadFile(path)
		if err == nil || attempt >= 25 || (!errors.Is(err, windows.ERROR_SHARING_VIOLATION) && !errors.Is(err, windows.ERROR_ACCESS_DENIED)) {
			return data, err
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// unlockFile releases the first-byte lock after the config commit.
func unlockFile(file *os.File) {
	_ = windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, &windows.Overlapped{})
}
