package toolkit

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"trpc.group/trpc-go/trpc-agent-go/tool"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

// testDefinition constructs a callable definition without any concrete application tool.
func testDefinition(name, category string) Definition {
	return Definition{Meta: Meta{Name: name, Category: category}, Factory: func(scope Scope) (tool.Tool, error) {
		return function.NewFunctionTool(func(context.Context, struct{}) (uint, error) { return scope.SessionID, nil }, function.WithName(name)), nil
	}}
}

// registerDefinition fails a test immediately when its registration is invalid.
func registerDefinition(t *testing.T, r Registrar, def Definition) {
	t.Helper()
	if err := r.Register(def); err != nil {
		t.Fatal(err)
	}
}

// TestRegistrySelectionAndAvailability covers registration order, optional tools and session gates.
func TestRegistrySelectionAndAvailability(t *testing.T) {
	r := NewRegistry()
	registerDefinition(t, r, testDefinition("first", CategoryBuiltin))
	optional := testDefinition("optional", CategoryUser)
	optional.Listed = true
	registerDefinition(t, r, optional)
	task := testDefinition("task", CategoryBuiltin)
	task.Available = func(scope Scope) bool { return scope.SessionKind == "task" }
	registerDefinition(t, r, task)
	if len(r.BuiltinTools()) != 2 || len(r.UserTools()) != 1 || len(r.ListedTools()) != 1 {
		t.Fatal("incorrect discovery categories")
	}
	if len(r.EnabledTools(nil)) != 2 {
		t.Fatal("discovery should preserve unavailable builtin metadata")
	}
	for _, tc := range []struct {
		scope Scope
		want  []string
	}{
		{Scope{}, []string{"first"}},
		{Scope{SessionKind: "task", EnabledTools: []string{"optional"}}, []string{"optional"}},
	} {
		built, err := r.BuildTools(tc.scope)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, current := range built {
			got = append(got, current.Declaration().Name)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("got %v, want %v", got, tc.want)
		}
	}
}

// TestRegistryRejectsInvalidAndDuplicateDefinitions prevents silent policy or factory replacement.
func TestRegistryRejectsInvalidAndDuplicateDefinitions(t *testing.T) {
	r := NewRegistry()
	for _, def := range []Definition{{}, {Meta: Meta{Name: "missing_factory", Category: CategoryBuiltin}}, testDefinition("invalid", "other")} {
		if err := r.Register(def); err == nil {
			t.Fatal("accepted invalid definition")
		}
	}
	def := testDefinition("one", CategoryBuiltin)
	registerDefinition(t, r, def)
	if err := r.Register(def); err == nil {
		t.Fatal("accepted duplicate definition")
	}
	if meta, ok := r.Get("one"); !ok || meta.Name != "one" {
		t.Fatal("registered metadata missing")
	}
}

// TestRegistryBuildsFreshScopedTools ensures constructors capture the requested session only.
func TestRegistryBuildsFreshScopedTools(t *testing.T) {
	r := NewRegistry()
	def := testDefinition("session", CategoryBuiltin)
	factory := def.Factory
	def.Factory = func(scope Scope) (tool.Tool, error) {
		// A factory may inspect its catalog without deadlocking on the registry mutex.
		if _, ok := r.Get("session"); !ok {
			return nil, errors.New("missing metadata")
		}
		return factory(scope)
	}
	registerDefinition(t, r, def)
	for _, id := range []uint{11, 22} {
		built, err := r.BuildTools(Scope{SessionID: id})
		if err != nil {
			t.Fatal(err)
		}
		result, err := built[0].(tool.CallableTool).Call(context.Background(), []byte(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		if result != id {
			t.Fatalf("session %d received %v", id, result)
		}
	}
}

// TestRegistryBuildErrors validates identity and preserves construction failures.
func TestRegistryBuildErrors(t *testing.T) {
	for _, mismatch := range []bool{false, true} {
		r := NewRegistry()
		def := testDefinition("expected", CategoryBuiltin)
		def.Factory = func(scope Scope) (tool.Tool, error) {
			if mismatch {
				return testDefinition("wrong", CategoryBuiltin).Factory(scope)
			}
			return nil, errors.New("service unavailable")
		}
		registerDefinition(t, r, def)
		if _, err := r.BuildTools(Scope{}); err == nil || !strings.Contains(err.Error(), "expected") {
			t.Fatalf("missing contextual error: %v", err)
		}
	}
}

// TestRegistryProviders verifies extension sources share the same ordered registration mechanism.
func TestRegistryProviders(t *testing.T) {
	r := NewRegistry()
	var calls []string
	for _, name := range []string{"mcp", "cli"} {
		if err := r.RegisterProvider(name, func(scope Scope) ([]tool.ToolSet, error) {
			if scope.SessionID != 42 {
				t.Fatal("scope was not forwarded")
			}
			calls = append(calls, name)
			return nil, nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.RegisterProvider("mcp", func(Scope) ([]tool.ToolSet, error) { return nil, nil }); err == nil {
		t.Fatal("accepted duplicate provider")
	}
	if _, err := r.BuildToolSets(Scope{SessionID: 42}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"mcp", "cli"}) {
		t.Fatalf("provider order: %v", calls)
	}
	if err := r.RegisterProvider("broken", func(Scope) ([]tool.ToolSet, error) { return nil, errors.New("offline") }); err != nil {
		t.Fatal(err)
	}
	if _, err := r.BuildToolSets(Scope{SessionID: 42}); err == nil || !strings.Contains(err.Error(), "broken") {
		t.Fatalf("provider error lost: %v", err)
	}
}

// TestSelectableBuiltinsHonorExplicitEmpty ensures visibility cannot bypass explicit selections.
func TestSelectableBuiltinsHonorExplicitEmpty(t *testing.T) {
	r := NewRegistry()
	public := testDefinition("public", CategoryBuiltin)
	public.Listed = true
	registerDefinition(t, r, public)
	registerDefinition(t, r, testDefinition("internal", CategoryBuiltin))
	for _, tc := range []struct {
		ids  []string
		want int
	}{{nil, 2}, {[]string{}, 0}, {[]string{"public"}, 1}, {[]string{"internal"}, 1}} {
		got, err := r.BuildTools(Scope{EnabledTools: tc.ids})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != tc.want {
			t.Fatalf("selection %#v built %d tools, want %d", tc.ids, len(got), tc.want)
		}
		if len(r.EnabledTools(tc.ids)) != tc.want {
			t.Fatalf("discovery does not match explicit selection %#v", tc.ids)
		}
	}
}
