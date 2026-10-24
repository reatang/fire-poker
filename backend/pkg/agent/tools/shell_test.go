package tools

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
)

type fakeShellProgressEmitter struct {
	sessionID uint
	toolName  string
	results   []string
}

// EmitToolResult records streamed shell progress for assertions.
func (f *fakeShellProgressEmitter) EmitToolResult(sessionID uint, toolName, result string) {
	f.sessionID = sessionID
	f.toolName = toolName
	f.results = append(f.results, result)
}

// TestShellFuncStreamsProgressAndReturnsCombinedOutput verifies pipe-mode progress.
func TestShellFuncStreamsProgressAndReturnsCombinedOutput(t *testing.T) {
	emitter := &fakeShellProgressEmitter{}
	command := `printf 'hello'; sleep 0.1; printf ' world'; printf ' warn' >&2`
	wantStdout, wantStderr := "hello world", " warn"
	if runtime.GOOS == "windows" {
		command = `echo hello world&echo warn 1>&2`
		wantStdout, wantStderr = "hello world\r\n", "warn \r\n"
	}
	out, err := shellFunc(context.Background(), emitter, 9, shellInput{
		Command: command,
	})
	if err != nil {
		t.Fatalf("shellFunc error: %v", err)
	}
	if out.Stdout != wantStdout {
		t.Fatalf("unexpected stdout: %q", out.Stdout)
	}
	if out.Stderr != wantStderr {
		t.Fatalf("unexpected stderr: %q", out.Stderr)
	}
	if emitter.sessionID != 9 || emitter.toolName != "shell" {
		t.Fatalf("unexpected emitter target: session=%d tool=%q", emitter.sessionID, emitter.toolName)
	}
	if len(emitter.results) == 0 {
		t.Fatal("expected streamed progress results")
	}
	if !strings.Contains(emitter.results[len(emitter.results)-1], "hello world") {
		t.Fatalf("expected final streamed result to include stdout, got %q", emitter.results[len(emitter.results)-1])
	}
}

// TestShellFuncPTYStreamsTerminalOutput verifies PTY mode still returns captured output.
func TestShellFuncPTYStreamsTerminalOutput(t *testing.T) {
	emitter := &fakeShellProgressEmitter{}
	command := `printf 'qr'; sleep 0.1; printf ' ready'`
	if runtime.GOOS == "windows" {
		command = `echo qr ready`
	}
	out, err := shellFunc(context.Background(), emitter, 10, shellInput{
		Command: command,
		UsePty:  true,
	})
	if err != nil {
		t.Fatalf("shellFunc pty error: %v", err)
	}
	if !strings.Contains(out.Stdout, "qr ready") {
		t.Fatalf("expected pty stdout to include terminal output, got %q", out.Stdout)
	}
	if out.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", out.ExitCode)
	}
	if len(emitter.results) == 0 {
		t.Fatal("expected streamed pty progress results")
	}
	if !strings.Contains(emitter.results[len(emitter.results)-1], "qr ready") {
		t.Fatalf("expected final pty progress to include output, got %q", emitter.results[len(emitter.results)-1])
	}
}

type fakeTerminalRunner struct {
	createParams terminalCreateParams
	chunks       []terminalOutputChunk
	visibility   []bool
}

// CreateTerminal records terminal creation parameters for assertions.
func (f *fakeTerminalRunner) CreateTerminal(ctx context.Context, params terminalCreateParams) (terminalInfo, error) {
	_ = ctx
	f.createParams = params
	return terminalInfo{ID: "term_shell", Status: "active"}, nil
}

// WaitTerminal simulates a completed terminal process.
func (f *fakeTerminalRunner) WaitTerminal(ctx context.Context, terminalID string) (terminalInfo, error) {
	_ = ctx
	if terminalID != "term_shell" {
		return terminalInfo{}, fmt.Errorf("unexpected terminal id %s", terminalID)
	}
	return terminalInfo{ID: terminalID, Status: "done", ExitCode: intPtr(0)}, nil
}

// ReadTerminalOutput returns canned terminal output chunks.
func (f *fakeTerminalRunner) ReadTerminalOutput(ctx context.Context, terminalID string, cursor int64) ([]terminalOutputChunk, error) {
	_ = ctx
	_ = terminalID
	_ = cursor
	return f.chunks, nil
}

// SetTerminalVisible records terminal visibility changes.
func (f *fakeTerminalRunner) SetTerminalVisible(ctx context.Context, terminalID string, visible bool, title string) error {
	_ = ctx
	_ = terminalID
	_ = title
	f.visibility = append(f.visibility, visible)
	return nil
}

// TestShellFuncUsesTerminalRunnerWhenAvailable verifies shell uses the shared backend.
func TestShellFuncUsesTerminalRunnerWhenAvailable(t *testing.T) {
	t.Setenv("SHELL", "/custom/shell")
	t.Setenv("ComSpec", "custom-shell.exe")
	runner := &fakeTerminalRunner{
		chunks: []terminalOutputChunk{{Data: "hello from terminal"}},
	}

	out, err := shellFuncWithTerminal(context.Background(), nil, 9, shellInput{
		Command: "printf hello",
	}, runner)
	if err != nil {
		t.Fatalf("shellFuncWithTerminal: %v", err)
	}
	if out.TerminalID != "term_shell" {
		t.Fatalf("terminal id=%q", out.TerminalID)
	}
	wantShell, wantArgs := "/custom/shell", []string{"-lc", "printf hello"}
	if runtime.GOOS == "windows" {
		wantShell, wantArgs = "custom-shell.exe", []string{"/d", "/s", "/c", "printf hello"}
	}
	if runner.createParams.Command != wantShell || strings.Join(runner.createParams.Args, "|") != strings.Join(wantArgs, "|") {
		t.Fatalf("unexpected create params: %+v", runner.createParams)
	}
	if out.Stdout != "hello from terminal" {
		t.Fatalf("stdout=%q", out.Stdout)
	}
}

// TestShellFuncFallsBackFromMissingWorkDir verifies model-supplied container paths do not break host commands.
func TestShellFuncFallsBackFromMissingWorkDir(t *testing.T) {
	missingDir := t.TempDir() + "/missing"
	out, err := shellFunc(context.Background(), nil, 0, shellInput{
		Command: "printf ok",
		WorkDir: missingDir,
	})
	if err != nil {
		t.Fatalf("shellFunc with missing work_dir should fall back: %v", err)
	}
	if out.Stdout != "ok" {
		t.Fatalf("stdout=%q", out.Stdout)
	}
}

// TestShellFuncTerminalHidesNormalCommandsByDefault prevents normal commands from showing panels.
func TestShellFuncTerminalHidesNormalCommandsByDefault(t *testing.T) {
	runner := &fakeTerminalRunner{
		chunks: []terminalOutputChunk{{Data: "plain command"}},
	}

	_, err := shellFuncWithTerminal(context.Background(), nil, 9, shellInput{
		Command: "printf hello",
	}, runner)
	if err != nil {
		t.Fatalf("shellFuncWithTerminal: %v", err)
	}
	if runner.createParams.Visible {
		t.Fatal("expected normal shell terminal to stay hidden by default")
	}
}

// TestShellFuncTerminalHidesVisibleTerminalAfterExit verifies completed visible terminals are hidden.
func TestShellFuncTerminalHidesVisibleTerminalAfterExit(t *testing.T) {
	runner := &fakeTerminalRunner{
		chunks: []terminalOutputChunk{{Data: "login done"}},
	}

	_, err := shellFuncWithTerminal(context.Background(), nil, 9, shellInput{
		Command: "login",
		UsePty:  true,
	}, runner)
	if err != nil {
		t.Fatalf("shellFuncWithTerminal: %v", err)
	}
	if !runner.createParams.Visible {
		t.Fatal("expected pty shell terminal to be visible while running")
	}
	if len(runner.visibility) != 1 || runner.visibility[0] {
		t.Fatalf("expected visible terminal to be hidden after exit, got %v", runner.visibility)
	}
}

// intPtr returns a pointer to a test integer value.
func intPtr(v int) *int { return &v }

// TestResolveShellTimeoutUsesDefaultsAndCaps verifies shell timeout normalization.
func TestResolveShellTimeoutUsesDefaultsAndCaps(t *testing.T) {
	if got := resolveShellTimeout(0); got != defaultShellTimeout {
		t.Fatalf("expected default timeout, got %v", got)
	}
	if got := resolveShellTimeout(int(maxShellTimeout.Seconds()) + 50); got != maxShellTimeout {
		t.Fatalf("expected capped timeout, got %v", got)
	}
}
