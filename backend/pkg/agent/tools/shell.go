package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/reatang/fire-poker/backend/pkg/agent/toolkit"
	pkgterminal "github.com/reatang/fire-poker/backend/pkg/terminal"
	"trpc.group/trpc-go/trpc-agent-go/tool/function"
)

const (
	defaultShellTimeout = 10 * time.Minute
	maxShellTimeout     = time.Hour
)

type ShellProgressEmitter interface {
	EmitToolResult(sessionID uint, toolName, result string)
}

type shellInput struct {
	Command        string `json:"command" jsonschema:"description=Shell command to execute,required"`
	WorkDir        string `json:"work_dir" jsonschema:"description=Working directory (default: user home)"`
	TimeoutSeconds int    `json:"timeout_seconds" jsonschema:"description=Optional timeout in seconds; defaults to 600 and is capped at 3600"`
	UsePty         bool   `json:"use_pty" jsonschema:"description=Run in a pseudo-terminal for interactive commands that draw terminal UI, print QR codes, or wait for browser/scan login"`
	ShowTerminal   bool   `json:"show_terminal" jsonschema:"description=Show the terminal panel to the user. Defaults to false for normal commands and true when use_pty is true"`
	KeepTerminal   bool   `json:"keep_terminal" jsonschema:"description=Keep a shown terminal visible after the command exits. Defaults to false"`
}

type shellOutput struct {
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	TerminalID string `json:"terminal_id,omitempty"`
}

type shellChunk struct {
	stream string
	text   string
}

// resolveShellTimeout applies the default and maximum timeout for shell runs.
func resolveShellTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		return defaultShellTimeout
	}
	timeout := time.Duration(seconds) * time.Second
	if timeout > maxShellTimeout {
		return maxShellTimeout
	}
	return timeout
}

func resolveShellCommand(command string) (string, []string) {
	return resolvePlatformShellCommand(runtime.GOOS, command, os.Getenv, exec.LookPath)
}

// Resolve within the host's shell family; Windows never silently switches to cmd syntax.
func resolvePlatformShellCommand(goos, command string, getenv func(string) string, lookPath func(string) (string, error)) (string, []string) {
	candidates := []string{"bash", "/bin/bash", "sh", "/bin/sh"}
	args := []string{"-lc", command}
	if goos == "darwin" {
		candidates = []string{"zsh", "/bin/zsh", "bash", "/bin/bash", "sh", "/bin/sh"}
	}
	if goos == "windows" {
		candidates = []string{"pwsh.exe"}
		for _, key := range []string{"ProgramW6432", "ProgramFiles", "LOCALAPPDATA"} {
			if root := getenv(key); root != "" {
				candidates = append(candidates, filepath.Join(root, "PowerShell", "7", "pwsh.exe"))
			}
		}
		candidates = append(candidates, "powershell.exe")
		if root := getenv("SystemRoot"); root != "" {
			candidates = append(candidates, filepath.Join(root, "System32", "WindowsPowerShell", "v1.0", "powershell.exe"))
		}
		args = []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-Command", command}
	}
	for _, candidate := range candidates {
		if resolved, err := lookPath(candidate); err == nil {
			return resolved, args
		}
	}
	// Let process creation report the missing native shell instead of using a foreign tool.
	return candidates[0], args
}

func resolveShellWorkDir(workDir string) string {
	if isDir(workDir) {
		return workDir
	}
	if home, err := os.UserHomeDir(); err == nil && isDir(home) {
		return home
	}
	return ""
}

func isDir(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// shellFunc executes a shell command without a shared terminal manager.
func shellFunc(ctx context.Context, emitter ShellProgressEmitter, sessionID uint, input shellInput) (shellOutput, error) {
	return shellFuncWithTerminal(ctx, emitter, sessionID, input, nil)
}

type terminalCreateParams = pkgterminal.CreateParams
type terminalInfo = pkgterminal.Info
type terminalOutputChunk = pkgterminal.TerminalOutputChunk

type ShellTerminalRunner interface {
	CreateTerminal(ctx context.Context, params terminalCreateParams) (terminalInfo, error)
	WaitTerminal(ctx context.Context, terminalID string) (terminalInfo, error)
	ReadTerminalOutput(ctx context.Context, terminalID string, cursor int64) ([]terminalOutputChunk, error)
	SetTerminalVisible(ctx context.Context, terminalID string, visible bool, title string) error
}

// shellFuncWithTerminal routes shell execution through the shared terminal manager when present.
func shellFuncWithTerminal(ctx context.Context, emitter ShellProgressEmitter, sessionID uint, input shellInput, terminalRunner ShellTerminalRunner) (shellOutput, error) {
	timeout := resolveShellTimeout(input.TimeoutSeconds)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if terminalRunner != nil {
		return shellFuncTerminal(ctx, emitter, sessionID, input, terminalRunner)
	}
	if input.UsePty {
		return shellFuncPTY(ctx, emitter, sessionID, input)
	}
	return shellFuncPipe(ctx, emitter, sessionID, input)
}

// shellFuncTerminal runs a shell command through the persistent interactive terminal backend.
func shellFuncTerminal(ctx context.Context, emitter ShellProgressEmitter, sessionID uint, input shellInput, terminalRunner ShellTerminalRunner) (shellOutput, error) {
	visible := input.ShowTerminal || input.UsePty
	shell, args := resolveShellCommand(input.Command)
	info, err := terminalRunner.CreateTerminal(ctx, terminalCreateParams{
		SessionID: sessionID,
		Title:     "Shell command",
		Command:   shell,
		Args:      args,
		Cwd:       resolveShellWorkDir(input.WorkDir),
		Visible:   visible,
	})
	if err != nil {
		return shellOutput{}, err
	}
	finalInfo, err := terminalRunner.WaitTerminal(ctx, info.ID)
	chunks, readErr := terminalRunner.ReadTerminalOutput(context.Background(), info.ID, 0)
	if readErr != nil && err == nil {
		err = readErr
	}
	if visible && !input.KeepTerminal {
		if hideErr := terminalRunner.SetTerminalVisible(context.Background(), info.ID, false, ""); hideErr != nil && err == nil {
			err = hideErr
		}
	}
	var stdout strings.Builder
	for _, chunk := range chunks {
		stdout.WriteString(chunk.Data)
	}
	output := stdout.String()
	if emitter != nil && sessionID != 0 {
		emitter.EmitToolResult(sessionID, toolkit.PlatformShellToolID(), output)
	}
	exitCode := 0
	if finalInfo.ExitCode != nil {
		exitCode = *finalInfo.ExitCode
	}
	return shellOutput{
		Stdout:     output,
		Stderr:     "",
		ExitCode:   exitCode,
		TerminalID: info.ID,
	}, err
}

// shellFuncPipe runs a non-PTY command with stdout and stderr pipes.
func shellFuncPipe(ctx context.Context, emitter ShellProgressEmitter, sessionID uint, input shellInput) (shellOutput, error) {
	shell, args := resolveShellCommand(input.Command)
	cmd := exec.CommandContext(ctx, shell, args...)
	cmd.Dir = resolveShellWorkDir(input.WorkDir)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return shellOutput{}, fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return shellOutput{}, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return shellOutput{}, fmt.Errorf("start command: %w", err)
	}

	chunks := make(chan shellChunk, 32)
	var wg sync.WaitGroup
	wg.Add(2)
	go readShellPipe(&wg, stdoutPipe, "stdout", chunks)
	go readShellPipe(&wg, stderrPipe, "stderr", chunks)
	go func() {
		wg.Wait()
		close(chunks)
	}()

	var stdout strings.Builder
	var stderr strings.Builder
	for chunk := range chunks {
		switch chunk.stream {
		case "stdout":
			stdout.WriteString(chunk.text)
		case "stderr":
			stderr.WriteString(chunk.text)
		}
		if emitter != nil && sessionID != 0 {
			emitter.EmitToolResult(sessionID, toolkit.PlatformShellToolID(), formatShellProgress(stdout.String(), stderr.String()))
		}
	}

	err = cmd.Wait()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() != nil {
			return shellOutput{
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
				ExitCode: -1,
			}, fmt.Errorf("exec: %w", ctx.Err())
		} else {
			return shellOutput{}, fmt.Errorf("exec: %w", err)
		}
	}

	return shellOutput{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}, nil
}

// readShellPipe copies one command output stream into shellChunk messages.
func readShellPipe(wg *sync.WaitGroup, r io.ReadCloser, stream string, chunks chan<- shellChunk) {
	defer wg.Done()
	defer r.Close()

	buf := make([]byte, 2048)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			chunks <- shellChunk{stream: stream, text: string(buf[:n])}
		}
		if err != nil {
			return
		}
	}
}

// formatShellProgress combines stdout and stderr for streamed tool progress.
func formatShellProgress(stdout, stderr string) string {
	switch {
	case stdout == "" && stderr == "":
		return ""
	case stderr == "":
		return stdout
	case stdout == "":
		return "[stderr]\n" + stderr
	default:
		return stdout + "\n[stderr]\n" + stderr
	}
}

// NewShellTool builds the agent tool that executes shell commands.
func NewShellTool(emitter ShellProgressEmitter, sessionID uint, terminalRunner ...ShellTerminalRunner) *function.FunctionTool[shellInput, shellOutput] {
	switch runtime.GOOS {
	case "windows":
		return NewWindowsShellTool(emitter, sessionID, terminalRunner...)
	case "darwin":
		return NewMacOSShellTool(emitter, sessionID, terminalRunner...)
	case "linux":
		return NewLinuxShellTool(emitter, sessionID, terminalRunner...)
	default:
		return newPlatformShellTool(runtime.GOOS, emitter, sessionID, terminalRunner...)
	}
}

func newPlatformShellTool(goos string, emitter ShellProgressEmitter, sessionID uint, terminalRunner ...ShellTerminalRunner) *function.FunctionTool[shellInput, shellOutput] {
	var runner ShellTerminalRunner
	if len(terminalRunner) > 0 {
		runner = terminalRunner[0]
	}
	return function.NewFunctionTool(
		func(ctx context.Context, input shellInput) (shellOutput, error) {
			if goos != runtime.GOOS || platformShellName(goos) == "" {
				return shellOutput{}, fmt.Errorf("%s shell tool is disabled on %s", goos, runtime.GOOS)
			}
			return shellFuncWithTerminal(ctx, emitter, sessionID, input, runner)
		},
		function.WithName(platformShellName(goos)),
		function.WithDescription(platformShellDescription(goos)+" Do not ask the user to write shell commands. Commands run in the shared interactive terminal backend when available. Set use_pty=true for interactive commands. Set show_terminal=true only when the terminal should be visible, and keep_terminal=true only when it should remain visible after exit."),
	)
}

// ShellMeta returns metadata for presenting the shell tool in confirmations.
func ShellMeta() ToolMeta {
	return ToolMeta{
		Name:            toolkit.PlatformShellToolID(),
		Description:     platformShellDescription(runtime.GOOS),
		Category:        CategoryBuiltin,
		RequiresConfirm: true,
		FormatPurpose: func(args json.RawMessage) string {
			var input shellInput
			_ = json.Unmarshal(args, &input)
			return fmt.Sprintf("Execute command: %s", input.Command)
		},
	}
}
