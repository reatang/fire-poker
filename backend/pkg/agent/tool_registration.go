package agent

import "github.com/reatang/fire-poker/backend/pkg/agent/tools"

// registerDefaultTools is the composition root; the catalog owns all tool selection and construction.
func (m *Manager) registerDefaultTools() error {
	if err := tools.RegisterDefaults(m.toolRegistry, func() tools.Dependencies {
		m.mu.RLock()
		defer m.mu.RUnlock()
		deps := tools.Dependencies{
			Emitter: toolResultEmitter{app: m.wailsApp},
			Skills:  m.skillProvider, Attention: m.attention, SkillCreator: m.skillCreator,
			CliInstaller: m.cliInstaller, CliInstallProgress: m.cliInstallProgress,
			CliManifestGenerator: m.cliManifestGenerator, CliCommandRunner: m.cliCommandRunner,
			TaskState: m.istorage,
		}
		// Do not put a typed nil terminal pointer inside the service interface.
		if m.terminalRunner != nil {
			deps.Terminal = m.terminalRunner
		}
		return deps
	}); err != nil {
		return err
	}
	return m.registerExtensionProviders(m.toolRegistry)
}
