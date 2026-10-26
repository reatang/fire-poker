// Package systemprompt stores reusable UTF-8 system prompts on disk.
package systemprompt

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

const DefaultDir = "./data/system-prompts"
const MaxBytes = 128 << 10

// ErrInvalidName rejects empty, overlong, path-like, special or Windows-reserved prompt names.
var ErrInvalidName = errors.New("prompt name must be 1-80 characters without paths, special characters or Windows reserved names")

// ErrInvalidContent rejects prompts that are not valid UTF-8 or exceed MaxBytes.
var ErrInvalidContent = errors.New("prompt must be valid UTF-8 text no larger than 128 KB")

type Prompt struct {
	Name   string `json:"name"`
	System string `json:"system"`
}

type Store struct {
	dir string
	mu  sync.RWMutex
}

func New(dir string) (*Store, error) {
	if dir == "" {
		dir = DefaultDir
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

func validateName(name string) error {
	if name == "" || utf8.RuneCountInString(name) > 80 || !utf8.ValidString(name) ||
		strings.TrimSpace(name) != name || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") ||
		strings.ContainsAny(name, `/\:<>"|?*`) || strings.ContainsFunc(name, unicode.IsControl) {
		return ErrInvalidName
	}
	base := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	switch base {
	case "CON", "PRN", "AUX", "NUL", "CONIN$", "CONOUT$":
		return ErrInvalidName
	}
	if len([]rune(base)) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) &&
		strings.ContainsRune("123456789¹²³", []rune(base)[3]) {
		return ErrInvalidName
	}
	return nil
}

func (s *Store) List() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".txt")
		if validateName(name) == nil {
			names = append(names, name)
		}
	}
	return names, nil
}

func (s *Store) Get(name string) (Prompt, error) {
	if err := validateName(name); err != nil {
		return Prompt{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	root, err := os.OpenRoot(s.dir)
	if err != nil {
		return Prompt{}, err
	}
	defer root.Close()
	info, err := root.Lstat(name + ".txt")
	if err != nil {
		return Prompt{}, err
	}
	if !info.Mode().IsRegular() {
		return Prompt{}, os.ErrNotExist
	}
	file, err := root.Open(name + ".txt")
	if err != nil {
		return Prompt{}, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxBytes+1))
	if err != nil {
		return Prompt{}, err
	}
	if len(data) > MaxBytes || !utf8.Valid(data) {
		return Prompt{}, ErrInvalidContent
	}
	return Prompt{Name: name, System: strings.TrimPrefix(string(data), "\uFEFF")}, nil
}

func (s *Store) Save(name, system string) (Prompt, error) {
	if err := validateName(name); err != nil {
		return Prompt{}, err
	}
	if len(system) > MaxBytes || !utf8.ValidString(system) {
		return Prompt{}, ErrInvalidContent
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	root, err := os.OpenRoot(s.dir)
	if err != nil {
		return Prompt{}, err
	}
	defer root.Close()
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return Prompt{}, err
	}
	temp := ".prompt-" + hex.EncodeToString(id[:]) + ".tmp"
	file, err := root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return Prompt{}, err
	}
	defer root.Remove(temp)
	_, writeErr := io.WriteString(file, system)
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return Prompt{}, err
	}
	if err := root.Rename(temp, name+".txt"); err != nil {
		return Prompt{}, fmt.Errorf("save prompt: %w", err)
	}
	return Prompt{Name: name, System: system}, nil
}
