package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/reatang/fire-poker/backend/models/data_models"
	pkgAgent "github.com/reatang/fire-poker/backend/pkg/agent"
	"github.com/reatang/fire-poker/backend/pkg/id/prompt_id"
	"github.com/reatang/fire-poker/backend/pkg/ierror"
	"github.com/reatang/fire-poker/backend/pkg/prompt"
	"github.com/reatang/fire-poker/backend/service/agent/agent_dto"
	"github.com/reatang/fire-poker/backend/storage"
)

// Agent exposes backend agent operations to the frontend.
type Agent struct {
	manager *pkgAgent.Manager
}

// Dependencies groups optional agent integrations that are only needed at app startup.
type Dependencies = pkgAgent.ManagerDeps

// NewAgent creates an agent service without optional tool services.
func NewAgent(istorage *storage.Storage) *Agent {
	return NewAgentWithDeps(istorage, Dependencies{})
}

// NewAgentWithDeps creates an agent service wired to the application's tool services.
func NewAgentWithDeps(istorage *storage.Storage, deps Dependencies) *Agent {
	return &Agent{manager: pkgAgent.NewManagerWithDeps(istorage, deps)}
}

func (a *Agent) SendMessage(ctx context.Context, input agent_dto.SendMessageInput) (*agent_dto.SendMessageOutput, error) {
	atts, err := convertAttachments(input.Attachments)
	if err != nil {
		return nil, ierror.Error(ierror.ErrAgentSendMessage, err)
	}

	handler := a.manager.NewChatHandler()
	err = handler.SendMessage(ctx, pkgAgent.SendMessageParams{
		RegenerateMessageID: input.RegenerateMessageID,
		SessionID:           input.SessionID,
		Content:             input.Content,
		SystemPrompt:        input.SystemPrompt,
		PrimedSkill:         input.SkillName,
		ProviderID:          input.ProviderID,
		ModelName:           input.ModelName,
		EnabledUserTools:    input.EnabledUserTools,
		Attachments:         atts,
	})
	if err != nil {
		return nil, agentError(ierror.ErrAgentSendMessage, err)
	}
	return &agent_dto.SendMessageOutput{}, nil
}

func (a *Agent) StopGeneration(ctx context.Context, input agent_dto.StopGenerationInput) (*agent_dto.StopGenerationOutput, error) {
	if err := a.manager.Streams().StopAndWait(ctx, input.SessionID); err != nil {
		return nil, err
	}
	_ = a.manager.Storage().UpdateSessionStatus(input.SessionID, "idle")
	return &agent_dto.StopGenerationOutput{}, nil
}

func (a *Agent) RespondToConfirm(ctx context.Context, input agent_dto.RespondToConfirmInput) (*agent_dto.RespondToConfirmOutput, error) {
	ok := a.manager.Streams().SendConfirmResponse(input.SessionID, pkgAgent.ConfirmResponse{
		Approved: input.Approved,
		Message:  input.Message,
		Action:   input.Action,
	})
	if !ok {
		return nil, ierror.Error(ierror.ErrAgentNoConfirm, fmt.Errorf("no active confirmation for session %d", input.SessionID))
	}
	return &agent_dto.RespondToConfirmOutput{}, nil
}

func (a *Agent) CreateSession(ctx context.Context, input agent_dto.CreateSessionInput) (*agent_dto.CreateSessionOutput, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = "New Chat"
	}
	tags := normalizeSessionTags(input.Tags)
	kind := "user"
	if tagsContain(tags, "task") {
		kind = "task"
	}

	ids, err := a.manager.DefaultToolIDs()
	if err != nil {
		return nil, ierror.Error(ierror.ErrAgentCreateSession, err)
	}
	toolData, _ := json.Marshal(ids)
	session, err := a.manager.Storage().CreateSession(data_models.Session{
		ToolIDs: string(toolData),
		Title:   title,
		Status:  "idle",
		Kind:    kind,
		Tags:    marshalSessionTags(tags),
	})
	if err != nil {
		return nil, ierror.Error(ierror.ErrAgentCreateSession, err)
	}

	return &agent_dto.CreateSessionOutput{
		SessionID: session.ID,
		Title:     session.Title,
		Tags:      tags,
	}, nil
}

func (a *Agent) ListSessions(ctx context.Context, input agent_dto.ListSessionsInput) (*agent_dto.ListSessionsOutput, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}

	sessions, err := a.manager.Storage().ListSessions(input.Cursor, limit+1, input.StarredOnly, input.IncludeHidden)
	if err != nil {
		return nil, ierror.Error(ierror.ErrAgentListSessions, err)
	}

	hasMore := len(sessions) > limit
	if hasMore {
		sessions = sessions[:limit]
	}

	items := make([]agent_dto.SessionItem, 0, len(sessions))
	for _, session := range sessions {
		items = append(items, toSessionItem(session))
	}

	var nextCursor uint
	if hasMore && len(sessions) > 0 {
		nextCursor = sessions[len(sessions)-1].ID
	}

	return &agent_dto.ListSessionsOutput{
		Sessions:   items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// SpawnTaskSession creates a task session and asynchronously submits its first turn.
func (a *Agent) SpawnTaskSession(ctx context.Context, input agent_dto.SpawnTaskSessionInput) (*agent_dto.SpawnTaskSessionOutput, error) {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = "Task"
	}

	ids, err := a.manager.DefaultToolIDs()
	if err != nil {
		return nil, ierror.Error(ierror.ErrAgentCreateSession, err)
	}
	toolData, _ := json.Marshal(ids)
	session, err := a.manager.Storage().CreateSession(data_models.Session{
		ToolIDs: string(toolData),
		Title:   title,
		Status:  "idle",
		Kind:    "task",
		Tags:    marshalSessionTags([]string{"task"}),
	})
	if err != nil {
		return nil, ierror.Error(ierror.ErrAgentCreateSession, err)
	}

	pkgAgent.EmitSessionSpawned(a.manager.App(), pkgAgent.SessionSpawnedEvent{
		SessionID:   session.ID,
		Title:       session.Title,
		Kind:        session.Kind,
		Tags:        []string{"task"},
		UserMessage: input.UserMessage,
	})

	go func() {
		handler := a.manager.NewChatHandler()
		_ = handler.SendMessage(context.Background(), pkgAgent.SendMessageParams{
			SessionID:    session.ID,
			Content:      input.UserMessage,
			SystemPrompt: input.SystemPrompt,
			PrimedSkill:  input.SkillName,
			ProviderID:   input.ProviderID,
			ModelName:    input.ModelName,
		})
	}()

	return &agent_dto.SpawnTaskSessionOutput{
		SessionID: session.ID,
		Title:     session.Title,
	}, nil
}

func (a *Agent) LoadSessionMessages(ctx context.Context, input agent_dto.LoadSessionMessagesInput) (*agent_dto.LoadSessionMessagesOutput, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}

	msgs, err := a.manager.Storage().ListMessagesForSession(input.SessionID, input.Offset, limit)
	if err != nil {
		return nil, ierror.Error(ierror.ErrAgentListMessages, err)
	}

	total, err := a.manager.Storage().CountMessagesForSession(input.SessionID)
	if err != nil {
		return nil, ierror.Error(ierror.ErrAgentCountMessages, err)
	}

	items := make([]agent_dto.MessageItem, 0, len(msgs))
	for _, msg := range msgs {
		items = append(items, toMessageItem(msg))
	}

	return &agent_dto.LoadSessionMessagesOutput{
		Messages: items,
		Total:    total,
		HasMore:  int64(input.Offset+len(msgs)) < total,
	}, nil
}

func (a *Agent) MarkSessionRead(ctx context.Context, input agent_dto.MarkSessionReadInput) (*agent_dto.MarkSessionReadOutput, error) {
	if err := a.manager.Storage().UpdateSessionStatus(input.SessionID, "idle"); err != nil {
		return nil, ierror.Error(ierror.ErrAgentMarkRead, err)
	}
	return &agent_dto.MarkSessionReadOutput{}, nil
}

func (a *Agent) RenameSession(ctx context.Context, input agent_dto.RenameSessionInput) (*agent_dto.RenameSessionOutput, error) {
	if err := a.manager.Storage().UpdateSessionTitle(input.SessionID, strings.TrimSpace(input.Title)); err != nil {
		return nil, ierror.Error(ierror.ErrAgentRename, err)
	}
	return &agent_dto.RenameSessionOutput{}, nil
}

func (a *Agent) DeleteSession(ctx context.Context, input agent_dto.DeleteSessionInput) (*agent_dto.DeleteSessionOutput, error) {
	if err := a.manager.Streams().StopAndWait(ctx, input.SessionID); err != nil {
		return nil, err
	}
	if err := a.manager.Storage().DeleteSession(input.SessionID); err != nil {
		return nil, ierror.Error(ierror.ErrAgentDelete, err)
	}
	return &agent_dto.DeleteSessionOutput{}, nil
}

func (a *Agent) ToggleStarSession(ctx context.Context, input agent_dto.ToggleStarSessionInput) (*agent_dto.ToggleStarSessionOutput, error) {
	if err := a.manager.Storage().UpdateSessionStarred(input.SessionID, input.Starred); err != nil {
		return nil, ierror.Error(ierror.ErrAgentToggleStar, err)
	}
	return &agent_dto.ToggleStarSessionOutput{}, nil
}

// GenerateTitle summarizes the opening turns into a session title with a single tool-free completion.
func (a *Agent) GenerateTitle(ctx context.Context, input agent_dto.GenerateTitleInput) (*agent_dto.GenerateTitleOutput, error) {
	msgs, err := a.manager.Storage().ListMessagesForSession(input.SessionID, 0, 4)
	if err != nil || len(msgs) == 0 {
		return &agent_dto.GenerateTitleOutput{Title: "New Chat"}, nil
	}

	var preview strings.Builder
	for _, msg := range msgs {
		if msg.ContentType != "text" || msg.Content == "" {
			continue
		}
		content := msg.Content
		if len(content) > 200 {
			content = content[:200]
		}
		fmt.Fprintf(&preview, "%s: %s\n", msg.Role, content)
	}

	if preview.Len() == 0 {
		return &agent_dto.GenerateTitleOutput{Title: "New Chat"}, nil
	}

	cfg, err := a.manager.ResolveModelConfig(input.ProviderID, input.ModelName)
	if err != nil {
		log.Printf("generate title for session %d: %v", input.SessionID, err)
		return &agent_dto.GenerateTitleOutput{Title: "New Chat"}, nil
	}

	// A oneshot call binds no tools and creates no SDK session, so nothing lingers after the title returns.
	resp, err := pkgAgent.OneshotComplete(ctx, cfg.ApplyTo(pkgAgent.OneshotRequest{
		User:      buildTitlePrompt(preview.String()),
		MaxTokens: titleMaxTokens,
		Timeout:   titleTimeout,
	}))
	if err != nil {
		log.Printf("generate title for session %d: %v", input.SessionID, err)
		return &agent_dto.GenerateTitleOutput{Title: "New Chat"}, nil
	}

	result := strings.TrimSpace(resp.Text)
	if result == "" {
		result = "New Chat"
	}

	_ = a.manager.Storage().UpdateSessionTitle(input.SessionID, result)
	return &agent_dto.GenerateTitleOutput{Title: result}, nil
}

const (
	titleMaxTokens = 64
	titleTimeout   = 30 * time.Second
)

func buildTitlePrompt(preview string) string {
	titleInstruction, err := prompt.Load(prompt_id.GenChatTitle)
	if err != nil || strings.TrimSpace(titleInstruction) == "" {
		titleInstruction = "Generate a concise title under 20 characters for this conversation. Return only the title."
	}

	return fmt.Sprintf("%s\n\n%s", titleInstruction, preview)
}

const (
	maxAttachmentCount = 10
	maxAttachmentBytes = 20 * 1024 * 1024
)

// convertAttachments validates attachment count, file size, and existence,
// then converts DTOs into normalized pkgAgent.Attachment values.
func convertAttachments(in []agent_dto.AttachmentInput) ([]pkgAgent.Attachment, error) {
	if len(in) == 0 {
		return nil, nil
	}
	if len(in) > maxAttachmentCount {
		return nil, ierror.Error(ierror.ErrAgentTooManyAttachments, fmt.Errorf("too many attachments: %d (max %d)", len(in), maxAttachmentCount))
	}
	out := make([]pkgAgent.Attachment, 0, len(in))
	for _, item := range in {
		path := strings.TrimSpace(item.Path)
		if path == "" {
			return nil, ierror.Error(ierror.ErrAgentAttachmentPath, fmt.Errorf("attachment path is empty"))
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, ierror.Error(ierror.ErrAgentAttachmentNotFound, fmt.Errorf("attachment not found: %s", path))
		}
		if info.Size() > maxAttachmentBytes {
			return nil, ierror.Error(ierror.ErrAgentAttachmentSize, fmt.Errorf("attachment %q exceeds %d bytes", info.Name(), maxAttachmentBytes))
		}
		out = append(out, pkgAgent.NormalizeAttachment(pkgAgent.Attachment{
			Path: path,
			Name: item.Name,
			Mime: item.Mime,
		}))
	}
	return out, nil
}
