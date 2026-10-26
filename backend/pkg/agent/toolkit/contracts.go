// Package toolkit defines registration contracts independently of tool implementations.
package toolkit

import (
	"encoding/json"

	"trpc.group/trpc-go/trpc-agent-go/tool"
)

const (
	CategoryBuiltin = "builtin"
	CategoryUser    = "user"
)

// Meta describes a tool for discovery and execution confirmation.
type Meta struct {
	Name            string
	Description     string
	Category        string
	RequiresConfirm bool
	FormatPurpose   func(json.RawMessage) string
}

// Scope supplies per-session construction inputs without depending on an agent manager.
type Scope struct {
	SessionID    uint
	SessionKind  string
	EnabledTools []string
	// WorkDir is the conversation's working directory; empty falls back to each tool's own default.
	WorkDir string
}

// Factory constructs a fresh tool; nil means unavailable in the requested scope.
type Factory func(Scope) (tool.Tool, error)

// Definition keeps discovery, availability and construction in one registration.
type Definition struct {
	Meta      Meta
	Factory   Factory
	Available func(Scope) bool
	Listed    bool
}

// Provider builds dynamically discovered tool sets such as MCP or CLI extensions.
type Provider func(Scope) ([]tool.ToolSet, error)

// Registrar is the write interface consumed by implementations and composition roots.
type Registrar interface {
	Register(Definition) error
	RegisterProvider(string, Provider) error
}

// Manager is the implementation-independent catalog and construction interface.
type Manager interface {
	Registrar
	Get(string) (Meta, bool)
	BuiltinTools() []Meta
	UserTools() []Meta
	ListedTools() []Meta
	EnabledTools([]string) []Meta
	Version() uint64
	BuildTools(Scope) ([]tool.Tool, error)
	BuildToolSets(Scope) ([]tool.ToolSet, error)
}
