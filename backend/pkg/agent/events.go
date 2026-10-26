package agent

import (
	"github.com/reatang/fire-poker/backend/pkg/id/event_id"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// Chat events are registered with typed payloads so the binding generator exposes them to the
// frontend and Wails rejects a mismatched payload. A rejected payload is never delivered, so every
// chat event must be emitted through the typed helpers below (enforced by events_test.go).
func init() {
	application.RegisterEvent[SessionStatusEvent](event_id.AgentSessionStatus)
	application.RegisterEvent[SessionSpawnedEvent](event_id.AgentSessionSpawned)
	application.RegisterEvent[StreamChunkEvent](event_id.AgentStreamChunk)
	application.RegisterEvent[StreamToolCallEvent](event_id.AgentStreamToolCall)
	application.RegisterEvent[StreamToolResultEvent](event_id.AgentStreamToolResult)
	application.RegisterEvent[StreamConfirmRequestEvent](event_id.AgentStreamConfirmRequest)
	application.RegisterEvent[StreamDoneEvent](event_id.AgentStreamDone)
	application.RegisterEvent[StreamErrorEvent](event_id.AgentStreamError)
}

// SessionStatusEvent reports a session's lifecycle status, e.g. loading, done-unread or idle.
type SessionStatusEvent struct {
	SessionID uint   `json:"sessionId"`
	Status    string `json:"status"`
}

// SessionSpawnedEvent announces a task session created in the background.
type SessionSpawnedEvent struct {
	SessionID   uint     `json:"sessionId"`
	Title       string   `json:"title"`
	Kind        string   `json:"kind"`
	Tags        []string `json:"tags"`
	UserMessage string   `json:"userMessage"`
}

// StreamChunkEvent carries one streamed delta plus the accumulated content of its kind.
type StreamChunkEvent struct {
	SessionID   uint   `json:"sessionId"`
	Seq         int64  `json:"seq"`
	Delta       string `json:"delta"`
	Content     string `json:"content"`
	ContentType string `json:"contentType"`
}

// StreamToolCallEvent announces a tool call the model requested.
type StreamToolCallEvent struct {
	SessionID uint   `json:"sessionId"`
	ToolName  string `json:"toolName"`
	Args      string `json:"args"`
	Purpose   string `json:"purpose"`
}

// StreamToolResultEvent carries a tool's output back to the conversation view.
type StreamToolResultEvent struct {
	SessionID uint   `json:"sessionId"`
	ToolName  string `json:"toolName"`
	Result    string `json:"result"`
}

// StreamConfirmRequestEvent asks the user to approve a pending tool call.
type StreamConfirmRequestEvent struct {
	SessionID uint   `json:"sessionId"`
	RequestID string `json:"requestId"`
	ToolName  string `json:"toolName"`
	Args      string `json:"args"`
	Purpose   string `json:"purpose"`
}

// TokenUsage reports prompt and completion tokens for a finished turn.
type TokenUsage struct {
	Input  int `json:"input"`
	Output int `json:"output"`
}

// StreamDoneEvent marks the end of a turn, whether completed or interrupted.
type StreamDoneEvent struct {
	SessionID   uint       `json:"sessionId"`
	Interrupted bool       `json:"interrupted"`
	Usage       TokenUsage `json:"usage"`
}

// StreamErrorEvent reports a model or persistence failure during a turn.
type StreamErrorEvent struct {
	SessionID uint   `json:"sessionId"`
	Error     string `json:"error"`
}

// emitSessionStatus publishes a status change; a nil app (tests, early startup) is a no-op.
func emitSessionStatus(app *application.App, payload SessionStatusEvent) {
	if app != nil {
		app.Event.Emit(event_id.AgentSessionStatus, payload)
	}
}

// EmitSessionSpawned publishes a background task session to every window.
func EmitSessionSpawned(app *application.App, payload SessionSpawnedEvent) {
	if app != nil {
		app.Event.Emit(event_id.AgentSessionSpawned, payload)
	}
}

// emitStreamChunk publishes one streamed delta.
func emitStreamChunk(app *application.App, payload StreamChunkEvent) {
	if app != nil {
		app.Event.Emit(event_id.AgentStreamChunk, payload)
	}
}

// emitStreamToolCall publishes a requested tool call.
func emitStreamToolCall(app *application.App, payload StreamToolCallEvent) {
	if app != nil {
		app.Event.Emit(event_id.AgentStreamToolCall, payload)
	}
}

// emitStreamToolResult publishes a tool's output.
func emitStreamToolResult(app *application.App, payload StreamToolResultEvent) {
	if app != nil {
		app.Event.Emit(event_id.AgentStreamToolResult, payload)
	}
}

// emitStreamConfirmRequest asks the frontend to show a tool approval prompt.
func emitStreamConfirmRequest(app *application.App, payload StreamConfirmRequestEvent) {
	if app != nil {
		app.Event.Emit(event_id.AgentStreamConfirmRequest, payload)
	}
}

// emitStreamDone publishes the end of a turn.
func emitStreamDone(app *application.App, payload StreamDoneEvent) {
	if app != nil {
		app.Event.Emit(event_id.AgentStreamDone, payload)
	}
}

// emitStreamError publishes a turn failure.
func emitStreamError(app *application.App, payload StreamErrorEvent) {
	if app != nil {
		app.Event.Emit(event_id.AgentStreamError, payload)
	}
}
