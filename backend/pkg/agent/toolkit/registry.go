package toolkit

import (
	"fmt"
	"strings"
	"sync"

	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// Registry stores definitions and providers in deterministic registration order.
type Registry struct {
	mu            sync.RWMutex
	definitions   map[string]Definition
	order         []string
	providers     map[string]Provider
	providerOrder []string
}

var _ Manager = (*Registry)(nil)

// Version changes after every successful registration, invalidating cached tool assemblies.
func (r *Registry) Version() uint64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return uint64(len(r.definitions) + len(r.providers))
}

// NewRegistry creates an independent registry.
func NewRegistry() *Registry {
	return &Registry{definitions: make(map[string]Definition), providers: make(map[string]Provider)}
}

// Register atomically registers metadata and factory, rejecting duplicate names.
func (r *Registry) Register(def Definition) error {
	if strings.TrimSpace(def.Meta.Name) == "" || def.Factory == nil {
		return fmt.Errorf("tool registration requires a name and factory")
	}
	if def.Meta.Category != CategoryBuiltin && def.Meta.Category != CategoryUser {
		return fmt.Errorf("tool %q has invalid category %q", def.Meta.Name, def.Meta.Category)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.definitions[def.Meta.Name]; exists {
		return fmt.Errorf("tool %q is already registered", def.Meta.Name)
	}
	r.definitions[def.Meta.Name] = def
	r.order = append(r.order, def.Meta.Name)
	return nil
}

// RegisterProvider registers an extension source without coupling it to the manager.
func (r *Registry) RegisterProvider(name string, provider Provider) error {
	if strings.TrimSpace(name) == "" || provider == nil {
		return fmt.Errorf("tool provider registration requires a name and provider")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("tool provider %q is already registered", name)
	}
	r.providers[name] = provider
	r.providerOrder = append(r.providerOrder, name)
	return nil
}

// Get returns the metadata for one tool.
func (r *Registry) Get(name string) (Meta, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.definitions[name]
	return def.Meta, ok
}

// snapshot copies definitions so factories never execute under the registry lock.
func (r *Registry) snapshot() []Definition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Definition, 0, len(r.order))
	for _, name := range r.order {
		result = append(result, r.definitions[name])
	}
	return result
}

// metadata selects metadata in registration order.
func (r *Registry) metadata(include func(Definition) bool) []Meta {
	var result []Meta
	for _, def := range r.snapshot() {
		if include(def) {
			result = append(result, def.Meta)
		}
	}
	return result
}

// BuiltinTools includes builtin metadata even when its dependencies are unavailable.
func (r *Registry) BuiltinTools() []Meta {
	return r.metadata(func(def Definition) bool { return def.Meta.Category == CategoryBuiltin })
}

// UserTools lists optional tools.
func (r *Registry) UserTools() []Meta {
	return r.metadata(func(def Definition) bool { return def.Meta.Category == CategoryUser })
}

// ListedTools lists user-facing capabilities.
func (r *Registry) ListedTools() []Meta {
	return r.metadata(func(def Definition) bool { return def.Listed })
}

// selected treats an explicit selection as an allowlist, including unlisted tools.
// Only legacy callers without a selection retain the builtin defaults.
func selected(def Definition, enabled []string) bool {
	if enabled == nil {
		return def.Meta.Category == CategoryBuiltin
	}
	for _, name := range enabled {
		if name == "shell" {
			name = PlatformShellToolID()
		}
		if name == def.Meta.Name {
			return true
		}
	}
	return false
}

// EnabledTools uses the same selection rule as construction.
func (r *Registry) EnabledTools(enabled []string) []Meta {
	return r.metadata(func(def Definition) bool { return selected(def, enabled) })
}

// BuildTools constructs fresh instances and propagates errors with their tool identity.
func (r *Registry) BuildTools(scope Scope) ([]tool.Tool, error) {
	var result []tool.Tool
	for _, def := range r.snapshot() {
		if !selected(def, scope.EnabledTools) || (def.Available != nil && !def.Available(scope)) {
			continue
		}
		current, err := def.Factory(scope)
		if err != nil {
			return nil, fmt.Errorf("build tool %q: %w", def.Meta.Name, err)
		}
		if current == nil {
			continue
		}
		if current.Declaration() == nil || current.Declaration().Name != def.Meta.Name {
			return nil, fmt.Errorf("tool %q factory returned a mismatched declaration", def.Meta.Name)
		}
		result = append(result, current)
	}
	return result, nil
}

// BuildToolSets invokes extension sources outside the registry lock.
func (r *Registry) BuildToolSets(scope Scope) ([]tool.ToolSet, error) {
	r.mu.RLock()
	names := append([]string(nil), r.providerOrder...)
	providers := make([]Provider, 0, len(names))
	for _, name := range names {
		providers = append(providers, r.providers[name])
	}
	r.mu.RUnlock()
	var result []tool.ToolSet
	for i, provider := range providers {
		sets, err := provider(scope)
		if err != nil {
			return nil, fmt.Errorf("build tool provider %q: %w", names[i], err)
		}
		result = append(result, sets...)
	}
	return result, nil
}
