// Package configfile serializes config.json updates and preserves unrelated fields.
package configfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/reatang/fire-poker/backend/models/data_models"
	"github.com/reatang/fire-poker/backend/pkg/dir"
)

var configMu sync.Mutex

// ReadFile reads a complete config snapshot, tolerating transient Windows replacement conflicts.
func ReadFile(path string) ([]byte, error) {
	return readFile(path)
}

// Update reads the latest config and commits a mutation under one shared lock.
// Slow network or runtime discovery must finish before entering this transaction.
func Update(dataDir string, mutate func(map[string]json.RawMessage) error) error {
	configMu.Lock()
	defer configMu.Unlock()
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(dataDir, ".config.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := lockFile(lock); err != nil {
		return err
	}
	defer unlockFile(lock)
	path := filepath.Join(dataDir, dir.ConfigFileName)
	current := make(map[string]json.RawMessage)
	raw, err := ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(raw, &current); err != nil {
			return err
		}
		if current == nil {
			current = make(map[string]json.RawMessage)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := mutate(current); err != nil {
		return err
	}
	content, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(dataDir, ".config-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if _, err := temp.Write(content); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return replaceFile(temp.Name(), path)
}

// SaveFields merges only the caller's owned fields, never a stale whole-file snapshot.
// An empty field list is reserved for creating or migrating a complete config.
func SaveFields(dataDir string, config *data_models.Config, fields ...string) error {
	raw, err := json.Marshal(config)
	if err != nil {
		return err
	}
	var next map[string]json.RawMessage
	if err := json.Unmarshal(raw, &next); err != nil {
		return err
	}
	return Update(dataDir, func(current map[string]json.RawMessage) error {
		if len(fields) == 0 {
			for key, value := range next {
				current[key] = value
			}
			return nil
		}
		for _, key := range fields {
			if value, ok := next[key]; ok {
				current[key] = value
			} else {
				delete(current, key)
			}
		}
		return nil
	})
}

// Ensure creates defaults only if no other module has initialized the config yet.
func Ensure(dataDir string, config *data_models.Config) error {
	raw, err := json.Marshal(config)
	if err != nil {
		return err
	}
	return Update(dataDir, func(current map[string]json.RawMessage) error {
		if len(current) != 0 {
			return nil
		}
		return json.Unmarshal(raw, &current)
	})
}
