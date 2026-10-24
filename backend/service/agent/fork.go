package agent

import (
	"context"
	"fmt"
	"github.com/reatang/fire-poker/backend/service/agent/agent_dto"
)

func (a *Agent) ForkSession(ctx context.Context, input agent_dto.ForkSessionInput) (*agent_dto.ForkSessionOutput, error) {
	if input.SessionID == 0 || input.MessageID == 0 {
		return nil, fmt.Errorf("请选择已保存的消息")
	}
	fork, err := a.manager.Storage().ForkSession(input.SessionID, input.MessageID)
	if err != nil {
		return nil, fmt.Errorf("无法分叉会话：%w", err)
	}
	return &agent_dto.ForkSessionOutput{Session: toSessionItem(*fork)}, nil
}
