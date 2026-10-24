package tools

import (
	"testing"

	"github.com/reatang/fire-poker/backend/pkg/agent/toolkit"
)

// TestDefaultCatalogBuildsEveryRegisteredTool catches missing factories and declaration mismatches.
func TestDefaultCatalogBuildsEveryRegisteredTool(t *testing.T) {
	r := toolkit.NewRegistry()
	deps := Dependencies{
		Skills: stubProvider{}, Attention: &fakeAttentionRequester{}, SkillCreator: &fakeSkillCreator{},
		CliInstaller: &fakeInstaller{}, CliInstallProgress: &fakeCliInstallProgressReporter{},
		CliManifestGenerator: &fakeGenerator{}, CliCommandRunner: &fakeCliCommandRunner{},
		TaskState: &fakeTaskStateStore{},
	}
	if err := RegisterDefaults(r, func() Dependencies { return deps }); err != nil {
		t.Fatal(err)
	}
	var enabled []string
	for _, def := range DefaultDefinitions(nil) {
		enabled = append(enabled, def.Meta.Name)
	}
	built, err := r.BuildTools(toolkit.Scope{SessionKind: "task", SessionID: 7, EnabledTools: enabled})
	if err != nil {
		t.Fatal(err)
	}
	if len(built) != 20 {
		t.Fatalf("expected all 20 registered tools, got %d", len(built))
	}
	if len(r.BuiltinTools()) != 19 || len(r.UserTools()) != 1 {
		t.Fatal("changed builtin/optional tool classification")
	}
	listed := ListedToolMetas()
	if len(listed) != 7 || len(r.ListedTools()) != len(listed) {
		t.Fatal("discovery diverged from runtime catalog")
	}
	for i, meta := range listed {
		if r.ListedTools()[i].Name != meta.Name {
			t.Fatal("discovery order diverged")
		}
	}
}

// TestDefaultCatalogResolvesServicesAtBuildTime covers startup wiring after registration.
func TestDefaultCatalogResolvesServicesAtBuildTime(t *testing.T) {
	r := toolkit.NewRegistry()
	deps := Dependencies{}
	if err := RegisterDefaults(r, func() Dependencies { return deps }); err != nil {
		t.Fatal(err)
	}
	before, err := r.BuildTools(toolkit.Scope{SessionKind: "user"})
	if err != nil {
		t.Fatal(err)
	}
	deps.Skills = stubProvider{}
	after, err := r.BuildTools(toolkit.Scope{SessionKind: "user"})
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before)+1 {
		t.Fatal("late service wiring did not enable Skill")
	}
}
