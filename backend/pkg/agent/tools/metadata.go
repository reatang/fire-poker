package tools

import "github.com/reatang/fire-poker/backend/pkg/agent/toolkit"

// ToolMeta keeps tool metadata compatible while its contract lives outside the implementation package.
type ToolMeta = toolkit.Meta

const (
	CategoryBuiltin = toolkit.CategoryBuiltin
	CategoryUser    = toolkit.CategoryUser
)
