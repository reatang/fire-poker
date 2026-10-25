package runtime_state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/reatang/fire-poker/backend/pkg/dir"
)

// StateSnapshot represents the persisted Node runtime install state.
type StateSnapshot struct {
	State       string    `json:"state"`
	Version     string    `json:"version"`
	InstallDir  string    `json:"install_dir"`
	NodePath    string    `json:"node_path"`
	NpmPath     string    `json:"npm_path"`
	ErrorMsg    string    `json:"error_msg"`
	InstalledAt time.Time `json:"installed_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

const (
	// StateMissing indicates the runtime has not been downloaded yet.
	StateMissing = "missing"
	// RuntimeStateFileName is the file used to persist runtime download state.
	RuntimeStateFileName = "state.json"
)

// LoadPersistedState reads cache/runtime/node/state.json. Missing file yields a fresh missing state.
func LoadPersistedState() (StateSnapshot, error) {
	dataDir, err := dir.GetDataDir()
	if err != nil {
		return StateSnapshot{}, err
	}
	path := filepath.Join(dir.NodeRuntimeRoot(dataDir), RuntimeStateFileName)
	bytes, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return StateSnapshot{State: StateMissing}, nil
		}
		return StateSnapshot{}, err
	}

	var state StateSnapshot
	if err := json.Unmarshal(bytes, &state); err != nil {
		return StateSnapshot{}, err
	}
	if state.State == "" {
		state.State = StateMissing
	}
	return state, nil
}
