package agent

import (
	"context"

	"github.com/reatang/fire-poker/backend/pkg/ierror"
	"github.com/reatang/fire-poker/backend/service/agent/agent_dto"
)

// GetSessionToolSettings loads this session's independent tool selection.
func (a *Agent) GetSessionToolSettings(ctx context.Context, input agent_dto.SessionToolsInput) (*agent_dto.SessionToolsOutput, error) {
	ids, err := a.manager.SessionToolIDs(input.SessionID)
	if err != nil {
		return nil, ierror.Error(ierror.ErrAgentListSessions, err)
	}
	locked, err := a.manager.Storage().SessionToolsLocked(input.SessionID)
	if err != nil {
		return nil, ierror.Error(ierror.ErrAgentListSessions, err)
	}
	return &agent_dto.SessionToolsOutput{ToolIDs: ids, Locked: locked || a.manager.Streams().IsActive(input.SessionID)}, nil
}

// SaveSessionToolSettings saves a pre-conversation selection without modifying global defaults.
func (a *Agent) SaveSessionToolSettings(ctx context.Context, input agent_dto.SaveSessionToolsInput) (*agent_dto.SessionToolsOutput, error) {
	if err := a.manager.Streams().WhileInactive(input.SessionID, func() error {
		return a.manager.Storage().UpdateSessionTools(input.SessionID, input.ToolIDs)
	}); err != nil {
		return nil, agentError(ierror.ErrAgentListSessions, err)
	}
	return a.GetSessionToolSettings(ctx, agent_dto.SessionToolsInput{SessionID: input.SessionID})
}
