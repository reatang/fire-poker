package agent

import (
	"encoding/json"
	"fmt"

	"github.com/reatang/fire-poker/backend/pkg/agent/toolkit"
	"github.com/reatang/fire-poker/backend/pkg/agent/tools"
	"github.com/reatang/fire-poker/backend/pkg/configfile"
	"github.com/reatang/fire-poker/backend/storage"
)

// DefaultToolIDs loads global defaults without consulting or modifying any session.
func (m *Manager) DefaultToolIDs() ([]string, error) {
	config, err := configfile.LoadCurrent()
	if err != nil {
		return nil, err
	}
	if config.DefaultToolIDs == nil {
		return tools.DefaultToolIDs(), nil
	}
	return toolkit.NormalizeToolIDs(config.DefaultToolIDs), nil
}

// ToolsForTurn accepts initial selection only before history exists; subsequent turns retain the saved tools.
// The caller must reserve the turn with Streams().TryStart before resolving tools.
func (m *Manager) ToolsForTurn(sessionID uint, requested []string) ([]string, error) {
	saved, err := m.SessionToolIDs(sessionID)
	if err != nil {
		return nil, err
	}
	locked, err := m.Storage().SessionToolsLocked(sessionID)
	if err != nil {
		return nil, err
	}
	if locked {
		if requested != nil && !toolkit.SameToolIDs(requested, saved) {
			return nil, storage.ErrSessionToolsLocked
		}
		return saved, nil
	}
	if requested == nil {
		return saved, nil
	}
	requested = toolkit.NormalizeToolIDs(requested)
	if err := m.Storage().UpdateSessionTools(sessionID, requested); err != nil {
		return nil, err
	}
	return requested, nil
}

// SessionToolIDs resolves legacy sessions once and freezes a separate copy of global defaults.
func (m *Manager) SessionToolIDs(sessionID uint) ([]string, error) {
	session, err := m.Storage().GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	if session.ToolIDs != "" {
		var ids []string
		if err := json.Unmarshal([]byte(session.ToolIDs), &ids); err != nil {
			return nil, fmt.Errorf("decode session tools: %w", err)
		}
		return toolkit.NormalizeToolIDs(ids), nil
	}
	ids, err := m.DefaultToolIDs()
	if err != nil {
		return nil, err
	}
	// Initialize only a previously unset row, so concurrent saves cannot be overwritten.
	if err := m.Storage().InitializeSessionTools(sessionID, ids); err != nil {
		return nil, err
	}
	return m.SessionToolIDs(sessionID)
}
