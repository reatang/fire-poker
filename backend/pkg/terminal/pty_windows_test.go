//go:build windows

package terminal

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWindowsConPTYInputResizeEnvironmentAndExitCode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// ConPTY emits UTF-8, but the child shell still uses the machine's console
	// code page. Select UTF-8 explicitly so non-ASCII output also works on CI.
	params := processTestParams(`[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new(); $line = [Console]::ReadLine(); [Console]::Out.WriteLine("got:$line"); [Console]::Out.WriteLine($env:LEMONTEA_TERMINAL_TEST); if (-not [String]::Equals((Get-Location).Path, $env:FIRE_POKER_EXPECTED_CWD, [StringComparison]::OrdinalIgnoreCase)) { throw "unexpected cwd: $((Get-Location).Path)" }; [Console]::Out.WriteLine("cwd:ok"); exit 7`)
	params.Cwd = t.TempDir()
	// Windows runners may supply an 8.3 TEMP path (RUNNER~1), while
	// PowerShell expands Get-Location to its long name (runneradmin).
	wantCwd, err := filepath.EvalSymlinks(params.Cwd)
	if err != nil {
		t.Fatal(err)
	}
	// Compare in the child so ConPTY line wrapping cannot split a long path.
	params.Env = append(os.Environ(), "LEMONTEA_TERMINAL_TEST=柠檬茶", "FIRE_POKER_EXPECTED_CWD="+wantCwd)
	p, _, err := startPTYProcess(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err := p.Resize(32, 100); err != nil {
		t.Fatal(err)
	}
	if err := p.Write([]byte("lemon\r")); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	for chunk := range p.Output() {
		output.Write(chunk)
	}
	code, err := p.Wait()
	if err != nil || code != 7 {
		t.Fatalf("exit=%d err=%v output=%q", code, err, output.String())
	}
	for _, want := range []string{"got:lemon", "柠檬茶", "cwd:ok"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("missing %q in %q", want, output.String())
		}
	}
}
