package agent

import (
	"context"

	"github.com/reatang/fire-poker/backend/pkg/ierror"
	"github.com/reatang/fire-poker/backend/service/agent/agent_dto"
)

// StoreAttachments persists uploads before returning paths for preview and sending.
func (a *Agent) StoreAttachments(ctx context.Context, input agent_dto.StoreAttachmentsInput) (*agent_dto.StoreAttachmentsOutput, error) {
	atts, err := convertAttachments(input.Attachments)
	if err != nil {
		return nil, ierror.Error(ierror.ErrAgentSendMessage, err)
	}
	stored, err := a.manager.StoreSessionAttachments(input.SessionID, atts)
	if err != nil {
		return nil, ierror.Error(ierror.ErrAgentSendMessage, err)
	}
	return &agent_dto.StoreAttachmentsOutput{Attachments: stored}, nil
}
