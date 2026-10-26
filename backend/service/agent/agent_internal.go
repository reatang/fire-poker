package agent

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/reatang/fire-poker/backend/models/data_models"
	pkgAgent "github.com/reatang/fire-poker/backend/pkg/agent"
	"github.com/reatang/fire-poker/backend/pkg/ierror"
	"github.com/reatang/fire-poker/backend/pkg/systemprompt"
	"github.com/reatang/fire-poker/backend/service/agent/agent_dto"
	"github.com/reatang/fire-poker/backend/storage"
)

const timeFormat = time.RFC3339

// knownAgentErrors maps sentinel errors from lower layers to a specific localized message,
// so the user sees the actual reason instead of a generic "operation failed".
var knownAgentErrors = []struct {
	target error
	code   ierror.ErrorCode
}{
	{pkgAgent.ErrSessionBusy, ierror.ErrAgentSessionBusy},
	{storage.ErrSystemPromptLocked, ierror.ErrAgentPromptLocked},
	{storage.ErrSessionToolsLocked, ierror.ErrAgentToolsLocked},
	{pkgAgent.ErrProviderRequired, ierror.ErrAgentProviderUnavail},
	{pkgAgent.ErrProviderNotFound, ierror.ErrAgentProviderUnavail},
	{pkgAgent.ErrProviderDisabled, ierror.ErrAgentProviderUnavail},
	{systemprompt.ErrInvalidName, ierror.ErrAgentPromptInvalidName},
	{systemprompt.ErrInvalidContent, ierror.ErrAgentPromptInvalid},
}

// agentError wraps err in an IError, preferring a specific code for known causes over fallback.
func agentError(fallback ierror.ErrorCode, err error) error {
	for _, known := range knownAgentErrors {
		if errors.Is(err, known.target) {
			return ierror.Error(known.code, err)
		}
	}
	return ierror.Error(fallback, err)
}

func toSessionItem(s data_models.Session) agent_dto.SessionItem {
	tags := sessionTags(s)
	return agent_dto.SessionItem{
		ID:      s.ID,
		Title:   s.Title,
		Kind:    s.Kind,
		Tags:    tags,
		Starred: s.Starred,
		Status:  s.Status,
		Created: s.CreatedAt.Format(timeFormat),
		Updated: s.UpdatedAt.Format(timeFormat),
	}
}

func sessionTags(s data_models.Session) []string {
	tags := parseSessionTags(s.Tags)
	if len(tags) == 0 && s.Kind == "task" {
		return []string{"task"}
	}
	return tags
}

func normalizeSessionTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.ToLower(strings.TrimSpace(tag))
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	return out
}

func parseSessionTags(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return nil
	}
	return normalizeSessionTags(tags)
}

func marshalSessionTags(tags []string) string {
	tags = normalizeSessionTags(tags)
	if len(tags) == 0 {
		return ""
	}
	content, _ := json.Marshal(tags)
	return string(content)
}

func tagsContain(tags []string, target string) bool {
	target = strings.ToLower(strings.TrimSpace(target))
	for _, tag := range tags {
		if tag == target {
			return true
		}
	}
	return false
}

func toMessageItem(m data_models.Message) agent_dto.MessageItem {
	return agent_dto.MessageItem{
		ID:          m.ID,
		SessionID:   m.SessionID,
		ParentID:    m.ParentID,
		Role:        m.Role,
		ContentType: m.ContentType,
		Content:     m.Content,
		ModelName:   m.ModelName,
		AgentName:   m.AgentName,
		TokensIn:    m.TokensIn,
		TokensOut:   m.TokensOut,
		Extra:       m.Extra,
		Attachments: m.Attachments,
		Created:     m.CreatedAt.Format(timeFormat),
	}
}
