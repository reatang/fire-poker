package agent_dto

import pkgAgent "github.com/reatang/fire-poker/backend/pkg/agent"

type StoreAttachmentsInput struct {
	SessionID   uint              `json:"session_id"`
	Attachments []AttachmentInput `json:"attachments"`
}

type StoreAttachmentsOutput struct {
	Attachments []pkgAgent.Attachment `json:"attachments"`
}
