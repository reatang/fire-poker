package mcp

import (
	"testing"
	"time"
)

func TestStdioTransportConfigDefaultsPositiveTimeout(t *testing.T) {
	cfg := ServerConfig{
		Transport: "stdio",
		Command:   "uv",
		Args:      []string{"run", "mysql_mcp_server"},
	}

	transport := cfg.stdioTransportConfig()
	if transport.Timeout <= 0 {
		t.Fatalf("expected positive timeout for stdio MCP config, got %s", transport.Timeout)
	}
}

func TestStdioTransportConfigPreservesExplicitTimeout(t *testing.T) {
	cfg := ServerConfig{
		Transport: "stdio",
		Command:   "uv",
		Args:      []string{"run", "mysql_mcp_server"},
		Timeout:   45 * time.Second,
	}

	transport := cfg.stdioTransportConfig()
	if transport.Timeout != 45*time.Second {
		t.Fatalf("expected timeout to remain 45s, got %s", transport.Timeout)
	}
}

func TestStdioTransportConfigPassesEnvAndWorkingDirWithoutShell(t *testing.T) {
	cfg := ServerConfig{
		Transport:  "stdio",
		Command:    "npx",
		Args:       []string{"-y", "server-everything"},
		Env:        map[string]string{"API_KEY": `a "quoted" $value`},
		WorkingDir: "/srv/mcp",
	}

	params := cfg.stdioTransportConfig().ServerParams
	if params.Command != "npx" {
		t.Fatalf("expected command to run directly, got %q", params.Command)
	}
	if len(params.Args) != 2 || params.Args[0] != "-y" || params.Args[1] != "server-everything" {
		t.Fatalf("expected args to be preserved, got %#v", params.Args)
	}
	if params.Env["API_KEY"] != `a "quoted" $value` {
		t.Fatalf("expected env value verbatim, got %q", params.Env["API_KEY"])
	}
	if params.WorkingDir != "/srv/mcp" {
		t.Fatalf("expected working dir to be passed through, got %q", params.WorkingDir)
	}

	cfg.Env["API_KEY"] = "changed"
	if params.Env["API_KEY"] == "changed" {
		t.Fatal("expected transport env to be a copy of the config env")
	}
}
