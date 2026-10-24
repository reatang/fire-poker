package terminal_dto

import pkgterminal "github.com/reatang/fire-poker/backend/pkg/terminal"

// ReadTerminalOutputInput identifies the terminal and cursor to read from.
type ReadTerminalOutputInput struct {
	TerminalID string `json:"terminal_id"`
	Cursor     int64  `json:"cursor"`
}

// ReadTerminalOutputOutput returns terminal output chunks after the cursor.
type ReadTerminalOutputOutput struct {
	Chunks []pkgterminal.TerminalOutputChunk `json:"chunks"`
}
