package agent_dto

type ForkSessionInput struct {
	SessionID uint `json:"session_id"`
	MessageID uint `json:"message_id"`
}

type ForkSessionOutput struct {
	Session SessionItem `json:"session"`
}
