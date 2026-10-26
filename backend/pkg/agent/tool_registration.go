package agent

import "github.com/reatang/fire-poker/backend/pkg/agent/tools"

// registerDefaultTools is the composition root; the catalog owns all tool selection and construction.
func (m *Manager) registerDefaultTools() error {
	if err := tools.RegisterDefaults(m.toolRegistry, func() tools.Dependencies {
		deps := tools.Dependencies{
			Emitter: toolResultEmitter{app: m.App()},
			Skills:  m.deps.SkillProvider, Attention: m.deps.AttentionRequester, SkillCreator: m.deps.SkillCreator,
			CliInstaller: m.deps.CliInstaller, CliInstallProgress: m.deps.CliInstallProgress,
			CliManifestGenerator: m.deps.CliManifestGenerator, CliCommandRunner: m.deps.CliCommandRunner,
			TaskState: m.istorage,
		}
		// Do not put a typed nil terminal pointer inside the service interface.
		if m.deps.TerminalRunner != nil {
			deps.Terminal = m.deps.TerminalRunner
		}
		return deps
	}); err != nil {
		return err
	}
	return m.registerExtensionProviders(m.toolRegistry)
}
