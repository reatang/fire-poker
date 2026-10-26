package agent

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/reatang/fire-poker/backend/models/data_models"
	"github.com/reatang/fire-poker/backend/pkg/i18n"
	"github.com/reatang/fire-poker/backend/pkg/ierror"
	"github.com/reatang/fire-poker/backend/storage"
	agentpkg "trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/model"
	pkgSession "trpc.group/trpc-go/trpc-agent-go/session"
)

// ChatHandler processes chat requests and bridges runner events to Wails/storage.
type ChatHandler struct {
	manager *Manager
}

// SendMessageParams defines the inputs required to run a chat turn.
type SendMessageParams struct {
	RegenerateMessageID uint
	SessionID           uint
	Content             string
	SystemPrompt        string
	PrimedSkill         string
	// ProviderID and ModelName select the model; credentials are resolved from storage.
	ProviderID       uint
	ModelName        string
	EnabledUserTools []string
	Attachments      []Attachment

	// model is resolved by SendMessage and reused by the event consumer and memory encoder.
	model ModelConfig
}

// NewChatHandler creates a chat handler for the given manager.
func NewChatHandler(m *Manager) *ChatHandler {
	return &ChatHandler{manager: m}
}

// updateSessionStatus persists and broadcasts a session status change.
func (ch *ChatHandler) updateSessionStatus(sessionID uint, status string) {
	ch.manager.updateSessionStatus(sessionID, status)
}

func (ch *ChatHandler) loadSessionHistory(sessionID uint) ([]model.Message, error) {
	return ch.loadSessionHistoryBefore(sessionID, 0)
}

func (ch *ChatHandler) loadSessionHistoryBefore(sessionID, beforeID uint) ([]model.Message, error) {
	storedMessages, err := ch.manager.Storage().ListMessagesForSession(sessionID, 0, -1)
	if err != nil {
		return nil, fmt.Errorf("list session messages: %w", err)
	}

	history := make([]model.Message, 0, len(storedMessages))
	pending := make([]model.ToolCall, 0)
	flushPending := func() {
		for _, tc := range pending {
			history = append(history, model.Message{Role: model.RoleTool, ToolID: tc.ID, ToolName: tc.Function.Name, Content: "工具执行已中断或结果未保存，不能视为执行成功。"})
		}
		pending = nil
	}

	for _, stored := range storedMessages {
		if beforeID > 0 && stored.ID >= beforeID {
			continue
		}
		msg, ok := toModelMessage(stored)
		if !ok {
			continue
		}
		if msg.Role == model.RoleTool {
			matched := -1
			for i, tc := range pending {
				if (msg.ToolID != "" && tc.ID == msg.ToolID) || (msg.ToolID == "" && tc.Function.Name == msg.ToolName) {
					matched = i
					msg.ToolID = tc.ID
					break
				}
			}
			if matched < 0 {
				continue
			}
			pending = append(pending[:matched], pending[matched+1:]...)
		} else if len(msg.ToolCalls) == 0 {
			flushPending()
		}
		pending = append(pending, msg.ToolCalls...)
		if len(msg.ToolCalls) > 0 && len(history) > 0 {
			last := &history[len(history)-1]
			if last.Role == model.RoleAssistant && len(last.ToolCalls) > 0 {
				last.ToolCalls = append(last.ToolCalls, msg.ToolCalls...)
				continue
			}
		}
		history = append(history, msg)
	}
	flushPending()

	return history, nil
}

func toModelMessage(stored data_models.Message) (model.Message, bool) {
	switch stored.ContentType {
	case "text":
		if stored.Role == "user" {
			// Attachments are loaded lazily from disk on every replay by design:
			// the spec keeps attachments path-only, no caching.
			// Malformed JSON degrades silently to text-only, matching the
			// fallback pattern used by the tool_call arm below.
			atts, err := UnmarshalAttachments(stored.Attachments)
			if err == nil && len(atts) > 0 {
				return BuildUserMessage(stored.Content, atts), true
			}
			return model.NewUserMessage(stored.Content), true
		}
		return model.NewAssistantMessage(stored.Content), true
	case "thinking":
		// Keep reasoning for display, but replay completed/partial answer text.
		return model.Message{}, false
	case "tool_result":
		return model.Message{
			Role:     model.RoleTool,
			ToolName: stored.AgentName,
			ToolID:   stored.Extra,
			Content:  stored.Content,
		}, true
	case "confirm_response":
		return model.NewUserMessage(stored.Content), true
	case "tool_call":
		var payload struct {
			ID   string          `json:"id"`
			Name string          `json:"name"`
			Args json.RawMessage `json:"args"`
		}
		if err := json.Unmarshal([]byte(stored.Content), &payload); err != nil {
			return model.NewAssistantMessage(stored.Content), true
		}
		return model.Message{
			Role: model.RoleAssistant,
			ToolCalls: []model.ToolCall{
				{
					Type: "function",
					ID:   payload.ID,
					Function: model.FunctionDefinitionParam{
						Name:      payload.Name,
						Arguments: payload.Args,
					},
				},
			},
		}, true
	default:
		return model.Message{}, false
	}
}

func normalizeToolArguments(raw json.RawMessage) (string, bool) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return "{}", true
	}
	if !json.Valid(trimmed) {
		return "", false
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, trimmed); err != nil {
		return "", false
	}

	return compact.String(), true
}

func toolCallFingerprint(toolName, args string) string {
	if toolName == "" || args == "" {
		return ""
	}
	return toolName + "\x00" + args
}

// preparedTurn holds what prepareTurn loaded before anything is written.
type preparedTurn struct {
	// regenerationQuestionID is the original question being retried, or 0 for a new question.
	regenerationQuestionID uint
	history                []model.Message
}

// SendMessage starts a streaming run and consumes events asynchronously.
// Steps run in a fixed order: read-only preparation, prompt resolution, persistence, then the run.
func (ch *ChatHandler) SendMessage(ctx context.Context, params SendMessageParams) error {
	streamCtx, cancel, err := ch.manager.Streams().TryStart(context.WithoutCancel(ctx), params.SessionID)
	if err != nil {
		return err
	}
	started := false
	defer func() {
		if !started {
			cancel()
			ch.manager.Streams().Remove(params.SessionID)
		}
	}()

	turn, err := ch.prepareTurn(&params)
	if err != nil {
		return err
	}
	session, err := ch.resolveSessionPrompt(&params)
	if err != nil {
		return err
	}
	if turn.regenerationQuestionID == 0 {
		if err := ch.persistUserMessage(&params); err != nil {
			return err
		}
	}

	_ = ch.manager.Storage().TouchSession(params.SessionID)
	ch.updateSessionStatus(params.SessionID, "loading")

	if err := ch.startRun(streamCtx, cancel, params, turn, session); err != nil {
		return err
	}
	started = true
	return nil
}

// prepareTurn resolves credentials and loads the turn's inputs without writing anything,
// so an invalid provider or regeneration target leaves no orphaned user message.
func (ch *ChatHandler) prepareTurn(params *SendMessageParams) (preparedTurn, error) {
	var turn preparedTurn
	var err error
	params.model, err = ch.manager.ResolveModelConfig(params.ProviderID, params.ModelName)
	if err != nil {
		return turn, fmt.Errorf("resolve model: %w", err)
	}
	params.ModelName = params.model.ModelName

	if params.RegenerateMessageID > 0 {
		question, err := ch.manager.Storage().RegenerationTurn(params.SessionID, params.RegenerateMessageID)
		if err != nil {
			return turn, err
		}
		// A retry replays the original question; whatever content the caller sent is ignored.
		turn.regenerationQuestionID = question.ID
		params.Content = question.Content
		params.Attachments, err = UnmarshalAttachments(question.Attachments)
		if err != nil {
			return turn, fmt.Errorf("load original attachments: %w", err)
		}
	}
	turn.history, err = ch.loadSessionHistoryBefore(params.SessionID, turn.regenerationQuestionID)
	if err != nil {
		return turn, err
	}
	return turn, nil
}

// resolveSessionPrompt freezes the session's tool selection and system prompt on its first turn.
// A started conversation keeps its prompt; asking for a different one is rejected.
func (ch *ChatHandler) resolveSessionPrompt(params *SendMessageParams) (*data_models.Session, error) {
	stor := ch.manager.Storage()
	session, err := stor.GetSession(params.SessionID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	params.EnabledUserTools, err = ch.manager.ToolsForTurn(params.SessionID, params.EnabledUserTools)
	if err != nil {
		return nil, fmt.Errorf("resolve session tools: %w", err)
	}

	requested := strings.TrimSpace(params.SystemPrompt)
	if requested == "" && strings.TrimSpace(session.SystemPrompt) != "" {
		return session, nil
	}
	count, err := stor.CountMessagesForSession(params.SessionID)
	if err != nil {
		return nil, err
	}
	if requested != "" {
		effective := session.SystemPrompt
		if strings.TrimSpace(effective) == "" {
			effective = DefaultSystemPrompt()
		}
		if count > 0 && requested != strings.TrimSpace(effective) {
			return nil, storage.ErrSystemPromptLocked
		}
		if count == 0 {
			if err := stor.UpdateSessionPrompt(params.SessionID, params.SystemPrompt, i18n.TCurrent(storage.PromptNameCustomKey, nil)); err != nil {
				return nil, err
			}
			session.SystemPrompt = params.SystemPrompt
		}
	}
	if strings.TrimSpace(session.SystemPrompt) == "" && count == 0 {
		session.SystemPrompt = DefaultSystemPrompt()
		if err := stor.UpdateSessionPrompt(params.SessionID, session.SystemPrompt, i18n.TCurrent(storage.PromptNameDefaultKey, nil)); err != nil {
			return nil, err
		}
	}
	return session, nil
}

// persistUserMessage moves the attachments into the session's working directory, then stores the
// new question before the model sees it, so both the history and the model use the kept copies.
func (ch *ChatHandler) persistUserMessage(params *SendMessageParams) error {
	if len(params.Attachments) > 0 {
		workDir := ch.manager.SessionWorkDir(params.SessionID)
		if workDir == "" {
			return fmt.Errorf("session %d has no working directory for attachments", params.SessionID)
		}
		stored, err := storeSessionAttachments(workDir, params.Attachments)
		if err != nil {
			return err
		}
		params.Attachments = stored
	}
	attachmentsJSON, err := MarshalAttachments(params.Attachments)
	if err != nil {
		return fmt.Errorf("marshal attachments: %w", err)
	}
	_, err = ch.manager.Storage().CreateMessage(data_models.Message{
		SessionID:   params.SessionID,
		Role:        "user",
		ContentType: "text",
		Content:     params.Content,
		Attachments: attachmentsJSON,
	})
	if err != nil {
		return fmt.Errorf("save user message: %w", err)
	}
	return nil
}

// startRun launches the runner and hands its events to a background consumer.
// On failure it marks the session as errored; the caller releases the stream reservation.
func (ch *ChatHandler) startRun(streamCtx context.Context, cancel context.CancelFunc, params SendMessageParams, turn preparedTurn, session *data_models.Session) error {
	r, err := ch.manager.GetOrCreateRunner(params.model, params.EnabledUserTools, session.Kind, params.SessionID)
	if err != nil {
		ch.updateSessionStatus(params.SessionID, "error-unread")
		return fmt.Errorf("get runner: %w", err)
	}

	userID := "local"
	// SDK history is seeded only once. Use a fresh temporary SDK session per
	// turn so SQLite remains authoritative, including interrupted output.
	sessionIDStr := strconv.FormatUint(uint64(params.SessionID), 10) + "-" + rand.Text()
	deleteSDKSession := func() {
		_ = ch.manager.sessionService.DeleteSession(context.Background(), pkgSession.Key{AppName: "fire-poker", UserID: userID, SessionID: sessionIDStr})
	}

	runOptions := make([]agentpkg.RunOption, 0, 2)
	if len(turn.history) > 0 {
		runOptions = append(runOptions, agentpkg.WithMessages(turn.history))
	}

	runCtx := withRunContext(streamCtx, runContext{
		SessionID:    params.SessionID,
		ModelName:    params.model.ModelName,
		BaseURL:      params.model.BaseURL,
		APIKey:       params.model.APIKey,
		ProviderType: params.model.ProviderType,
	})

	basePrompt := session.SystemPrompt
	if strings.TrimSpace(params.SystemPrompt) != "" {
		basePrompt = params.SystemPrompt
	}
	if strings.TrimSpace(basePrompt) == "" {
		basePrompt = DefaultSystemPrompt()
	}
	runOptions = append(runOptions, agentpkg.WithInstruction(ch.memorySystemPrompt(basePrompt, params.Content)))
	userMsg := BuildUserMessage(buildRunContent("", params.PrimedSkill, params.Content), params.Attachments)

	events, err := r.Run(runCtx, userID, sessionIDStr, userMsg, runOptions...)
	if err != nil {
		cancel()
		ch.manager.Streams().Remove(params.SessionID)
		ch.updateSessionStatus(params.SessionID, "error-unread")
		return fmt.Errorf("runner.Run: %w", err)
	}

	if turn.regenerationQuestionID > 0 {
		// Archive only after the retry has started, so a failed start keeps the previous answer.
		if err := ch.manager.Storage().ArchiveRegeneratedResponse(params.SessionID, turn.regenerationQuestionID, params.RegenerateMessageID); err != nil {
			cancel()
			go func() {
				for range events {
				}
				deleteSDKSession()
			}()
			ch.updateSessionStatus(params.SessionID, "error-unread")
			return fmt.Errorf("archive previous response: %w", err)
		}
	}
	go func() {
		ch.consumeEventsWithParams(streamCtx, params, events)
		// Drain runner shutdown before removing its temporary session.
		for range events {
		}
		deleteSDKSession()
	}()
	return nil
}

// buildRunContent folds hidden-session bootstrap instructions into the first user turn
// without changing the shared runner's global prompt contract.
func buildRunContent(systemPrompt, primedSkill, content string) string {
	systemPrompt = strings.TrimSpace(systemPrompt)
	primedSkill = strings.TrimSpace(primedSkill)
	content = strings.TrimSpace(content)

	parts := make([]string, 0, 3)
	if systemPrompt != "" {
		parts = append(parts, "System instructions:\n"+systemPrompt)
	}
	if primedSkill != "" {
		parts = append(parts, "Before doing any other work, call the Skill tool with {\"name\":\""+primedSkill+"\"}.")
	}
	if content != "" {
		parts = append(parts, content)
	}
	return strings.Join(parts, "\n\n")
}

func (ch *ChatHandler) consumeEvents(ctx context.Context, sessionID uint, modelName string, events <-chan *event.Event) {
	ch.consumeEventsWithParams(ctx, SendMessageParams{SessionID: sessionID, ModelName: modelName}, events)
}

func (ch *ChatHandler) consumeEventsWithParams(ctx context.Context, params SendMessageParams, events <-chan *event.Event) {
	sessionID := params.SessionID
	modelName := params.ModelName
	defer ch.manager.Streams().Remove(sessionID)

	stor := ch.manager.Storage()
	var fullContent string
	var fullThinking string
	var tokensIn int
	var tokensOut int
	var streamSeq int64
	var primaryChoiceIndex *int
	failed := false
	defer func() {
		interrupted := ctx.Err() != nil
		extra := ""
		if interrupted {
			extra = "interrupted"
		} else if failed {
			extra = "incomplete"
		}
		var output []data_models.Message
		if fullThinking != "" {
			output = append(output, data_models.Message{SessionID: sessionID, Role: "assistant", ContentType: "thinking", Content: fullThinking, ModelName: modelName, Extra: extra})
		}
		if fullContent != "" {
			output = append(output, data_models.Message{SessionID: sessionID, Role: "assistant", ContentType: "text", Content: fullContent, ModelName: modelName, TokensIn: tokensIn, TokensOut: tokensOut, Extra: extra})
		}
		if err := stor.CreateMessages(&output); err != nil {
			log.Printf("persist session %d output: %v", sessionID, err)
			failed = true
			emitStreamError(ch.manager.App(), StreamErrorEvent{SessionID: sessionID, Error: ierror.ErrAgentSaveOutput.Msg() + ": " + err.Error()})
		}
		status := "done-unread"
		if interrupted {
			status = "idle"
		} else if failed {
			status = "error-unread"
		}
		ch.updateSessionStatus(sessionID, status)
		emitStreamDone(ch.manager.App(), StreamDoneEvent{SessionID: sessionID, Interrupted: interrupted, Usage: TokenUsage{Input: tokensIn, Output: tokensOut}})
		if !interrupted && !failed {
			ch.enqueueMemoryEncoding(params, fullContent)
		}
	}()
	handledToolCalls := make(map[string]struct{})
	lastToolCallFingerprint := ""

	emitChunk := func(delta, contentType, content string) {
		streamSeq++
		emitStreamChunk(ch.manager.App(), StreamChunkEvent{
			SessionID:   sessionID,
			Seq:         streamSeq,
			Delta:       delta,
			Content:     content,
			ContentType: contentType,
		})
	}

	for {
		var evt *event.Event
		select {
		case <-ctx.Done():
			return
		case next, ok := <-events:
			if !ok {
				return
			}
			evt = next
		}
		if evt == nil {
			continue
		}

		select {
		case <-ctx.Done():
			ch.updateSessionStatus(sessionID, "idle")
			return
		default:
		}

		if evt.Error != nil {
			failed = true
			emitStreamError(ch.manager.App(), StreamErrorEvent{SessionID: sessionID, Error: evt.Error.Message})
			ch.updateSessionStatus(sessionID, "error-unread")
			errContent, _ := json.Marshal(map[string]string{
				"msg":    ierror.ErrAgentStreamError.Msg(),
				"detail": evt.Error.Message,
			})
			_, _ = stor.CreateMessage(data_models.Message{
				SessionID:   sessionID,
				Role:        "assistant",
				ContentType: "error",
				Content:     string(errContent),
				ModelName:   modelName,
			})
			return
		}

		for _, choice := range evt.Choices {
			if primaryChoiceIndex == nil {
				idx := choice.Index
				primaryChoiceIndex = &idx
			}
			if primaryChoiceIndex != nil && choice.Index != *primaryChoiceIndex {
				continue
			}

			delta := choice.Delta
			message := choice.Message
			isToolResultMessage := message.ToolID != ""

			if !isToolResultMessage && delta.ReasoningContent != "" {
				lastToolCallFingerprint = ""
				fullThinking += delta.ReasoningContent
				emitChunk(delta.ReasoningContent, "thinking", fullThinking)
			}
			if !isToolResultMessage && delta.ReasoningContent == "" && message.ReasoningContent != "" && fullThinking == "" {
				lastToolCallFingerprint = ""
				fullThinking = message.ReasoningContent
				emitChunk(message.ReasoningContent, "thinking", fullThinking)
			}

			if !isToolResultMessage && delta.Content != "" {
				lastToolCallFingerprint = ""
				fullContent += delta.Content
				emitChunk(delta.Content, "text", fullContent)
			}
			if !isToolResultMessage && delta.Content == "" && message.Content != "" && fullContent == "" {
				lastToolCallFingerprint = ""
				fullContent = message.Content
				emitChunk(message.Content, "text", fullContent)
			}

			toolCalls := delta.ToolCalls
			if len(toolCalls) == 0 && len(message.ToolCalls) > 0 {
				toolCalls = message.ToolCalls
			}

			for _, tc := range toolCalls {
				normalizedArgs, ok := normalizeToolArguments(tc.Function.Arguments)
				if !ok {
					continue
				}

				if tc.ID != "" {
					if _, exists := handledToolCalls[tc.ID]; exists {
						continue
					}
				}

				toolName := tc.Function.Name
				fingerprint := toolCallFingerprint(toolName, normalizedArgs)
				if fingerprint != "" && fingerprint == lastToolCallFingerprint {
					if tc.ID != "" {
						handledToolCalls[tc.ID] = struct{}{}
					}
					continue
				}
				lastToolCallFingerprint = fingerprint
				if tc.ID != "" {
					handledToolCalls[tc.ID] = struct{}{}
				}

				args := normalizedArgs

				meta, hasMeta := ch.manager.ToolRegistry().Get(toolName)
				purpose := ""
				if hasMeta && meta.FormatPurpose != nil {
					purpose = meta.FormatPurpose(json.RawMessage(args))
				}

				payload, _ := json.Marshal(map[string]any{
					"id":   tc.ID,
					"name": toolName,
					"args": json.RawMessage(args),
				})

				emitStreamToolCall(ch.manager.App(), StreamToolCallEvent{
					SessionID: sessionID,
					ToolName:  toolName,
					Args:      args,
					Purpose:   purpose,
				})

				_, _ = stor.CreateMessage(data_models.Message{
					SessionID:   sessionID,
					Role:        "assistant",
					ContentType: "tool_call",
					Content:     string(payload),
					ModelName:   modelName,
					Extra:       purpose,
				})

			}

			if message.ToolID != "" {
				lastToolCallFingerprint = ""
				emitStreamToolResult(ch.manager.App(), StreamToolResultEvent{
					SessionID: sessionID,
					ToolName:  message.ToolName,
					Result:    message.Content,
				})
				_, _ = stor.CreateMessage(data_models.Message{
					SessionID:   sessionID,
					Role:        "tool",
					ContentType: "tool_result",
					Content:     message.Content,
					ModelName:   modelName,
					AgentName:   message.ToolName,
					Extra:       message.ToolID,
				})
			}
		}

		if evt.Usage != nil {
			tokensIn = evt.Usage.PromptTokens
			tokensOut = evt.Usage.CompletionTokens
		}

		if evt.Done {
			break
		}
	}

}
