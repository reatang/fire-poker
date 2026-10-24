package terminal

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

func processTestParams(command string) CreateParams {
	if runtime.GOOS == "windows" {
		shell, err := exec.LookPath("pwsh.exe")
		if err != nil {
			shell = "powershell.exe"
		}
		return CreateParams{Command: shell, Args: []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-Command", command}}
	}
	return CreateParams{Command: "sh", Args: []string{"-c", command}}
}

func TestPipeProcessInputAndExitCode(t *testing.T) {
	command := `read line; printf 'got:%s' "$line"; exit 7`
	if runtime.GOOS == "windows" {
		command = `$line = [Console]::ReadLine(); [Console]::Out.Write("got:$line"); exit 7`
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p, _, err := startPipeProcess(ctx, processTestParams(command))
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err := p.Write([]byte("lemon\n")); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	for chunk := range p.Output() {
		output.Write(chunk)
	}
	code, _ := p.Wait()
	if code != 7 || !strings.Contains(output.String(), "got:lemon") {
		t.Fatalf("exit=%d output=%q", code, output.String())
	}
}

func TestTerminalProcessesCancel(t *testing.T) {
	for _, pty := range []bool{false, true} {
		t.Run(map[bool]string{false: "pipes", true: "pty"}[pty], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			command := "sleep 30"
			if runtime.GOOS == "windows" {
				command = "Start-Sleep -Seconds 30"
			}
			start := startPipeProcess
			if pty {
				start = startPTYProcess
			}
			p, _, err := start(ctx, processTestParams(command))
			if err != nil {
				t.Fatal(err)
			}
			go func() {
				for range p.Output() {
				}
			}()
			// Allow the client to attach before cancelling its context.
			time.Sleep(500 * time.Millisecond)
			cancel()
			select {
			case <-p.Done():
			case <-time.After(5 * time.Second):
				t.Fatal("terminal did not stop after cancellation")
			}
		})
	}
}
