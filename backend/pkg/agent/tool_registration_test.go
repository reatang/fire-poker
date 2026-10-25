package agent

import (
	"context"
	"testing"

	"github.com/reatang/fire-poker/backend/pkg/agent/toolkit"
	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

// registerTestTool installs a test implementation through the same contract as production tools.
func registerTestTool(t *testing.T, registrar toolkit.Registrar, meta toolkit.Meta) {
	t.Helper()
	if err := registrar.Register(toolkit.Definition{Meta: meta, Factory: func(toolkit.Scope) (tool.Tool, error) {
		return function.NewFunctionTool(func(context.Context, struct{}) (string, error) { return "ok", nil }, function.WithName(meta.Name)), nil
	}}); err != nil {
		t.Fatal(err)
	}
}

// TestRegisteredToolNeedsNoManagerDispatch proves a new tool is usable without changing Agent code.
func TestRegisteredToolNeedsNoManagerDispatch(t *testing.T) {
	m := NewManager(nil)
	registerTestTool(t, m.ToolRegistry(), toolkit.Meta{Name: "custom", Category: toolkit.CategoryUser})
	built, err := m.buildAgentTools([]string{"custom"}, "user", 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, current := range built {
		if current.Declaration().Name == "custom" {
			return
		}
	}
	t.Fatal("registered custom tool was not constructed")
}

// unreachableModelConfig builds runners without ever contacting a model.
var unreachableModelConfig = ModelConfig{ProviderID: 1, BaseURL: "http://127.0.0.1:1", APIKey: "unused", ModelName: "test"}

// TestRunnerCacheSeparatesSessionScopedTools protects the per-session factory lifecycle.
func TestRunnerCacheSeparatesSessionScopedTools(t *testing.T) {
	m := NewManager(nil)
	first, err := m.GetOrCreateRunner(unreachableModelConfig, nil, "task", 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.GetOrCreateRunner(unreachableModelConfig, nil, "task", 2)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("session-bound tool instances were shared")
	}
}

// TestRunnerCacheTracksRegistrationAndServiceChanges ensures cached runners see new capabilities.
func TestRunnerCacheTracksRegistrationAndServiceChanges(t *testing.T) {
	m := NewManager(nil)
	first, err := m.GetOrCreateRunner(unreachableModelConfig, nil, "user", 1)
	if err != nil {
		t.Fatal(err)
	}
	same, err := m.GetOrCreateRunner(unreachableModelConfig, nil, "user", 1)
	if err != nil || same != first {
		t.Fatal("unchanged scope should reuse its runner")
	}
	registerTestTool(t, m.ToolRegistry(), toolkit.Meta{Name: "new_builtin", Category: toolkit.CategoryBuiltin})
	second, err := m.GetOrCreateRunner(unreachableModelConfig, nil, "user", 1)
	if err != nil {
		t.Fatal(err)
	}
	if second == first {
		t.Fatal("catalog registration did not invalidate runner assembly")
	}
	m.SetCliInstallProgressReporter(&fakeCliInstallProgressReporter{})
	third, err := m.GetOrCreateRunner(unreachableModelConfig, nil, "user", 1)
	if err != nil {
		t.Fatal(err)
	}
	if third == second {
		t.Fatal("service wiring did not invalidate runner assembly")
	}
}

// TestRunnerCacheDistinguishesUnsetAndEmptySelection prevents an all-off selection reusing default tools.
func TestRunnerCacheDistinguishesUnsetAndEmptySelection(t *testing.T) {
	m := NewManager(nil)
	first, err := m.GetOrCreateRunner(unreachableModelConfig, nil, "user", 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.GetOrCreateRunner(unreachableModelConfig, []string{}, "user", 1)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("explicit empty selection reused default tool instances")
	}
}
