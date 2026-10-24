package agent

import (
	"context"
	"fmt"
	pkgAgent "github.com/reatang/fire-poker/backend/pkg/agent"
	"github.com/reatang/fire-poker/backend/pkg/dir"
	"github.com/reatang/fire-poker/backend/pkg/systemprompt"
	"github.com/reatang/fire-poker/backend/service/agent/agent_dto"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func (a *Agent) GetSessionPrompt(ctx context.Context, input agent_dto.SessionPromptInput) (*agent_dto.SessionPromptOutput, error) {
	s, err := a.manager.Storage().GetSession(input.SessionID)
	if err != nil {
		return nil, err
	}
	defaults := pkgAgent.DefaultSystemPrompt()
	value := s.SystemPrompt
	isDefault := strings.TrimSpace(value) == ""
	if isDefault {
		value = defaults
	}
	count, err := a.manager.Storage().CountMessagesForSession(input.SessionID)
	if err != nil {
		return nil, err
	}
	name := s.PromptName
	if name == "" {
		if isDefault {
			name = "默认系统提示词"
		} else {
			name = "自定义系统提示词"
		}
	}
	return &agent_dto.SessionPromptOutput{System: value, Name: name, DefaultSystem: defaults, IsDefault: isDefault, Locked: count > 0}, nil
}

func (a *Agent) UpdateSessionPrompt(ctx context.Context, input agent_dto.UpdateSessionPromptInput) (*agent_dto.SessionPromptOutput, error) {
	if a.manager.Streams().IsActive(input.SessionID) {
		return nil, fmt.Errorf("请先停止生成，再修改提示词")
	}
	if _, err := a.manager.Storage().GetSession(input.SessionID); err != nil {
		return nil, err
	}
	if input.Reset {
		input.System, input.Name = "", ""
	} else if strings.TrimSpace(input.System) == "" {
		return nil, fmt.Errorf("提示词不能为空；恢复默认请使用恢复默认按钮")
	}
	if len(input.System) > systemprompt.MaxBytes || !utf8.ValidString(input.System) {
		return nil, systemprompt.ErrInvalidContent
	}
	if utf8.RuneCountInString(input.Name) > 80 {
		return nil, fmt.Errorf("提示词名称不能超过 80 个字符")
	}
	if err := a.manager.Storage().UpdateSessionPrompt(input.SessionID, input.System, input.Name); err != nil {
		return nil, err
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

func (a *Agent) ListSystemPrompts(ctx context.Context) (*agent_dto.PromptLibraryOutput, error) {
	store, err := promptStore()
	if err != nil {
		return nil, err
	}
	names, err := store.List()
	if err != nil {
		return nil, err
	}
	if names == nil {
		names = []string{}
	}
	return &agent_dto.PromptLibraryOutput{Names: names}, nil
}

func (a *Agent) GetSystemPrompt(ctx context.Context, input agent_dto.PromptLibraryInput) (*agent_dto.PromptOutput, error) {
	store, err := promptStore()
	if err != nil {
		return nil, err
	}
	p, err := store.Get(input.Name)
	if err != nil {
		return nil, err
	}
	return &agent_dto.PromptOutput{Name: p.Name, System: p.System}, nil
}

func (a *Agent) SaveSystemPrompt(ctx context.Context, input agent_dto.PromptLibraryInput) (*agent_dto.PromptOutput, error) {
	if strings.TrimSpace(input.System) == "" {
		return nil, fmt.Errorf("提示词不能为空")
	}
	store, err := promptStore()
	if err != nil {
		return nil, err
	}
	p, err := store.Save(input.Name, input.System)
	if err != nil {
		return nil, err
	}
	return &agent_dto.PromptOutput{Name: p.Name, System: p.System}, nil
}
