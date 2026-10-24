//go:build !windows

package configfile

import (
	"golang.org/x/sys/unix"
	"os"
)

// lockFile excludes concurrent writers from other app instances sharing the data directory.
func lockFile(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_EX)
}

// unlockFile releases the file lock after the config commit.
func unlockFile(file *os.File) {
	_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
}

// replaceFile atomically replaces the config on the same filesystem.
func replaceFile(source, target string) error {
	return os.Rename(source, target)
}

// readFile reads the complete snapshot exposed by atomic replacement.
func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
