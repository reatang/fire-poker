package agent

import (
	"context"
	"errors"

	"github.com/reatang/fire-poker/backend/pkg/ierror"
	"github.com/reatang/fire-poker/backend/service/agent/agent_dto"
)

// ForkSession copies the conversation up to a saved message into a new session.
func (a *Agent) ForkSession(ctx context.Context, input agent_dto.ForkSessionInput) (*agent_dto.ForkSessionOutput, error) {
	if input.SessionID == 0 || input.MessageID == 0 {
		return nil, ierror.Error(ierror.ErrAgentForkTarget, errors.New("session and message are required"))
	}
	fork, err := a.manager.Storage().ForkSession(input.SessionID, input.MessageID)
	if err != nil {
		return nil, ierror.Error(ierror.ErrAgentFork, err)
	}
	return &agent_dto.ForkSessionOutput{Session: toSessionItem(*fork)}, nil
}
