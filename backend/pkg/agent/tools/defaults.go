package tools

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/reatang/fire-poker/backend/pkg/agent/toolkit"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// Dependencies contains typed service interfaces used by tool implementations.
type Dependencies struct {
	Emitter              ShellProgressEmitter
	Terminal             ShellTerminalRunner
	Skills               SkillProvider
	Attention            AttentionRequester
	SkillCreator         SkillCreator
	CliInstaller         CliInstaller
	CliInstallProgress   CliInstallProgressReporter
	CliManifestGenerator CliManifestGenerator
	CliCommandRunner     CliCommandRunner
	TaskState            TaskStateStore
}

// DefaultDefinitions is the single catalog of builtin and optional local tools.
// Dependencies are resolved at construction time, after application services are wired.
func DefaultDefinitions(resolve func() Dependencies) []toolkit.Definition {
	define := func(meta ToolMeta, listed bool, available func(toolkit.Scope, Dependencies) bool, factory func(toolkit.Scope, Dependencies) tool.Tool) toolkit.Definition {
		dependencies := func() Dependencies {
			if resolve == nil {
				return Dependencies{}
			}
			return resolve()
		}
		return toolkit.Definition{
			Meta: meta, Listed: listed,
			Factory: func(scope toolkit.Scope) (tool.Tool, error) {
				// Check availability and construct against the same service snapshot.
				d := dependencies()
				if !available(scope, d) {
					return nil, nil
				}
				return factory(scope, d), nil
			},
		}
	}
	return []toolkit.Definition{
		define(DateTimeMeta(), true, func(s toolkit.Scope, d Dependencies) bool { return true }, func(s toolkit.Scope, d Dependencies) tool.Tool { return NewDateTimeTool() }),
		define(FileReadMeta(), false, func(s toolkit.Scope, d Dependencies) bool { return true }, func(s toolkit.Scope, d Dependencies) tool.Tool { return NewFileReadTool() }),
		define(FileWriteMeta(), false, func(s toolkit.Scope, d Dependencies) bool { return true }, func(s toolkit.Scope, d Dependencies) tool.Tool { return NewFileWriteTool() }),
		define(ShellMeta(), true, func(s toolkit.Scope, d Dependencies) bool { return platformShellName(runtime.GOOS) != "" }, func(s toolkit.Scope, d Dependencies) tool.Tool {
			return NewShellTool(d.Emitter, s.SessionID, d.Terminal)
		}),
		define(BuildQuestionTool(), true, func(s toolkit.Scope, d Dependencies) bool { return true }, func(s toolkit.Scope, d Dependencies) tool.Tool { return NewQuestionTool() }),
		define(WebFetchMeta(), true, func(s toolkit.Scope, d Dependencies) bool { return true }, func(s toolkit.Scope, d Dependencies) tool.Tool { return NewWebFetchTool() }),
		define(BuildWebSearchTool(), true, func(s toolkit.Scope, d Dependencies) bool { return true }, func(s toolkit.Scope, d Dependencies) tool.Tool { return NewWebSearchTool() }),
		define(CodeExecMeta(), false, func(s toolkit.Scope, d Dependencies) bool { return true }, func(s toolkit.Scope, d Dependencies) tool.Tool { return NewCodeExecTool() }),
		define(BuildQRCodeTool(), false, func(s toolkit.Scope, d Dependencies) bool { return true }, func(s toolkit.Scope, d Dependencies) tool.Tool { return NewQRCodeTool() }),
		define(BuildInteractiveTerminalTool(), false, func(s toolkit.Scope, d Dependencies) bool { return true }, func(s toolkit.Scope, d Dependencies) tool.Tool { return NewInteractiveTerminalTool() }),
		define(BuildRequestAttentionTool(), false, func(s toolkit.Scope, d Dependencies) bool { return s.SessionKind == "task" && d.Attention != nil }, func(s toolkit.Scope, d Dependencies) tool.Tool {
			return NewRequestAttentionTool(d.Attention, s.SessionID)
		}),
		define(BuildProposeSkillTool(), false, func(s toolkit.Scope, d Dependencies) bool {
			return s.SessionKind == "task" && d.Attention != nil && d.SkillCreator != nil
		}, func(s toolkit.Scope, d Dependencies) tool.Tool {
			return NewProposeSkillTool(d.Attention, d.SkillCreator, s.SessionID)
		}),
		define(BuildInstallCliTool(), false, func(s toolkit.Scope, d Dependencies) bool { return d.CliInstaller != nil }, func(s toolkit.Scope, d Dependencies) tool.Tool { return NewInstallCliTool(d.CliInstaller) }),
		define(BuildReportCliInstallProgressTool(), false, func(s toolkit.Scope, d Dependencies) bool {
			return s.SessionKind == "task" && d.CliInstallProgress != nil
		}, func(s toolkit.Scope, d Dependencies) tool.Tool {
			return NewReportCliInstallProgressTool(d.CliInstallProgress, s.SessionID)
		}),
		define(BuildGenerateCliManifestTool(), false, func(s toolkit.Scope, d Dependencies) bool { return d.CliManifestGenerator != nil }, func(s toolkit.Scope, d Dependencies) tool.Tool {
			return NewGenerateCliManifestTool(d.CliManifestGenerator)
		}),
		define(BuildRunCliCommandTool(), false, func(s toolkit.Scope, d Dependencies) bool { return d.CliCommandRunner != nil }, func(s toolkit.Scope, d Dependencies) tool.Tool {
			return NewRunCliCommandTool(d.CliCommandRunner, d.Emitter, s.SessionID)
		}),
		define(BuildSaveTaskStateTool(), false, func(s toolkit.Scope, d Dependencies) bool { return s.SessionKind == "task" }, func(s toolkit.Scope, d Dependencies) tool.Tool { return NewSaveTaskStateTool(d.TaskState, s.SessionID) }),
		define(BuildLoadTaskStateTool(), false, func(s toolkit.Scope, d Dependencies) bool { return s.SessionKind == "task" }, func(s toolkit.Scope, d Dependencies) tool.Tool { return NewLoadTaskStateTool(d.TaskState, s.SessionID) }),
		define(BuildTodoWriteTool(), true, func(s toolkit.Scope, d Dependencies) bool { return true }, func(s toolkit.Scope, d Dependencies) tool.Tool { return NewTodoWriteTool() }),
		define(skillMeta(), true, func(s toolkit.Scope, d Dependencies) bool { return d.Skills != nil }, func(s toolkit.Scope, d Dependencies) tool.Tool { return NewSkillTool(d.Skills) }),
	}
}

// RegisterDefaults installs every local tool through the public registrar contract.
func RegisterDefaults(registrar toolkit.Registrar, resolve func() Dependencies) error {
	for _, def := range DefaultDefinitions(resolve) {
		if err := registrar.Register(def); err != nil {
			return fmt.Errorf("register default tools: %w", err)
		}
	}
	return nil
}

// ListedToolMetas derives user-facing discovery from the same definitions used at runtime.
func ListedToolMetas() []ToolMeta {
	var result []ToolMeta
	for _, def := range DefaultDefinitions(nil) {
		if def.Listed {
			result = append(result, def.Meta)
		}
	}
	return result
}

// DefaultToolIDs derives initial user-selectable defaults from the registered builtin catalog.
func DefaultToolIDs() []string {
	var ids []string
	for _, def := range DefaultDefinitions(nil) {
		if def.Listed && def.Meta.Category == CategoryBuiltin {
			ids = append(ids, def.Meta.Name)
		}
	}
	return ids
}

// skillMeta describes Skill before a provider is available to build its live description.
func skillMeta() ToolMeta {
	return ToolMeta{
		Name: SkillToolName, Description: "Load a skill's instructions by name", Category: CategoryBuiltin,
		FormatPurpose: func(args json.RawMessage) string {
			var input skillInput
			_ = json.Unmarshal(args, &input)
			return "Loading skill: " + input.Name
		},
	}
}
