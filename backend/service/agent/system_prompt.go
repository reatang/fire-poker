package agent

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"unicode/utf8"

	pkgAgent "github.com/reatang/fire-poker/backend/pkg/agent"
	"github.com/reatang/fire-poker/backend/pkg/dir"
	"github.com/reatang/fire-poker/backend/pkg/i18n"
	"github.com/reatang/fire-poker/backend/pkg/ierror"
	"github.com/reatang/fire-poker/backend/pkg/systemprompt"
	"github.com/reatang/fire-poker/backend/service/agent/agent_dto"
	"github.com/reatang/fire-poker/backend/storage"
)

// maxPromptNameRunes bounds the display name stored with a session's prompt.
const maxPromptNameRunes = 80

// GetSessionPrompt returns the session's effective system prompt and whether it is locked.
func (a *Agent) GetSessionPrompt(ctx context.Context, input agent_dto.SessionPromptInput) (*agent_dto.SessionPromptOutput, error) {
	s, err := a.manager.Storage().GetSession(input.SessionID)
	if err != nil {
		return nil, ierror.Error(ierror.ErrAgentListSessions, err)
	}
	defaults := pkgAgent.DefaultSystemPrompt()
	value := s.SystemPrompt
	isDefault := strings.TrimSpace(value) == ""
	if isDefault {
		value = defaults
	}
	count, err := a.manager.Storage().CountMessagesForSession(input.SessionID)
	if err != nil {
		return nil, ierror.Error(ierror.ErrAgentCountMessages, err)
	}
	name := s.PromptName
	if name == "" {
		if isDefault {
			name = i18n.TCurrent(storage.PromptNameDefaultKey, nil)
		} else {
			name = i18n.TCurrent(storage.PromptNameCustomKey, nil)
		}
	}
	return &agent_dto.SessionPromptOutput{System: value, Name: name, DefaultSystem: defaults, IsDefault: isDefault, Locked: count > 0}, nil
}

// UpdateSessionPrompt sets or resets the prompt of a session that has not started yet.
func (a *Agent) UpdateSessionPrompt(ctx context.Context, input agent_dto.UpdateSessionPromptInput) (*agent_dto.SessionPromptOutput, error) {
	if a.manager.Streams().IsActive(input.SessionID) {
		return nil, ierror.Error(ierror.ErrAgentStopBeforePrompt, pkgAgent.ErrSessionBusy)
	}
	if _, err := a.manager.Storage().GetSession(input.SessionID); err != nil {
		return nil, ierror.Error(ierror.ErrAgentUpdatePrompt, err)
	}
	if input.Reset {
		input.System, input.Name = "", ""
	} else if strings.TrimSpace(input.System) == "" {
		return nil, ierror.Error(ierror.ErrAgentPromptEmpty, errors.New("prompt is empty"))
	}
	if len(input.System) > systemprompt.MaxBytes || !utf8.ValidString(input.System) {
		return nil, ierror.Error(ierror.ErrAgentPromptInvalid, systemprompt.ErrInvalidContent)
	}
	if utf8.RuneCountInString(input.Name) > maxPromptNameRunes {
		return nil, ierror.Error(ierror.ErrAgentPromptNameTooLong, errors.New("prompt name is too long"))
	}
	if err := a.manager.Storage().UpdateSessionPrompt(input.SessionID, input.System, input.Name); err != nil {
		return nil, agentError(ierror.ErrAgentUpdatePrompt, err)
	}
	return a.GetSessionPrompt(ctx, agent_dto.SessionPromptInput{SessionID: input.SessionID})
}

func promptStore() (*systemprompt.Store, error) {
	root, err := dir.GetDataDir()
	if err != nil {
		return nil, err
	}
	return systemprompt.New(filepath.Join(root, "system-prompts"))
}

// ListSystemPrompts returns the names saved in the prompt library.
func (a *Agent) ListSystemPrompts(ctx context.Context) (*agent_dto.PromptLibraryOutput, error) {
	store, err := promptStore()
	if err != nil {
		return nil, ierror.Error(ierror.ErrAgentPromptLibrary, err)
	}
	names, err := store.List()
	if err != nil {
		return nil, ierror.Error(ierror.ErrAgentPromptLibrary, err)
	}
	if names == nil {
		names = []string{}
	}
	return &agent_dto.PromptLibraryOutput{Names: names}, nil
}

// GetSystemPrompt loads one prompt from the library.
func (a *Agent) GetSystemPrompt(ctx context.Context, input agent_dto.PromptLibraryInput) (*agent_dto.PromptOutput, error) {
	store, err := promptStore()
	if err != nil {
		return nil, ierror.Error(ierror.ErrAgentPromptLibrary, err)
	}
	p, err := store.Get(input.Name)
	if err != nil {
		return nil, agentError(ierror.ErrAgentPromptLibrary, err)
	}
	return &agent_dto.PromptOutput{Name: p.Name, System: p.System}, nil
}

// SaveSystemPrompt stores a non-empty prompt in the library under a validated name.
func (a *Agent) SaveSystemPrompt(ctx context.Context, input agent_dto.PromptLibraryInput) (*agent_dto.PromptOutput, error) {
	if strings.TrimSpace(input.System) == "" {
		return nil, ierror.Error(ierror.ErrAgentPromptEmpty, errors.New("prompt is empty"))
	}
	store, err := promptStore()
	if err != nil {
		return nil, ierror.Error(ierror.ErrAgentPromptLibrary, err)
	}
	p, err := store.Save(input.Name, input.System)
	if err != nil {
		return nil, agentError(ierror.ErrAgentPromptLibrary, err)
	}
	return &agent_dto.PromptOutput{Name: p.Name, System: p.System}, nil
}
