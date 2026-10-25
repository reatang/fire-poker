package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/reatang/fire-poker/backend/models/data_models"
	mcpgo "trpc.group/trpc-go/trpc-mcp-go"
)

// helperServerEnv makes the test binary act as a stdio MCP server, so the tests
// exercise a real child process without depending on node, npx or a shell.
const helperServerEnv = "FIRE_POKER_MCP_TEST_SERVER"

func TestMain(m *testing.M) {
	if os.Getenv(helperServerEnv) == "1" {
		runHelperServer()
		return
	}
	os.Exit(m.Run())
}

func runHelperServer() {
	server := mcpgo.NewStdioServer("helper", "1.0.0")
	server.RegisterTool(
		mcpgo.NewTool("probe",
			mcpgo.WithDescription("Report the process env value and working directory"),
			mcpgo.WithString("key", mcpgo.Required(), mcpgo.Description("Env var to read")),
		),
		func(ctx context.Context, req *mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			key, _ := req.Params.Arguments["key"].(string)
			cwd, err := os.Getwd()
			if err != nil {
				return nil, err
			}
			payload, err := json.Marshal(map[string]string{"value": os.Getenv(key), "cwd": cwd})
			if err != nil {
				return nil, err
			}
			return mcpgo.NewTextResult(string(payload)), nil
		},
	)
	server.RegisterTool(
		mcpgo.NewTool("other", mcpgo.WithDescription("Unselected tool")),
		func(ctx context.Context, req *mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			return mcpgo.NewTextResult("other"), nil
		},
	)
	if err := server.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func helperServerConfig(t *testing.T, workingDir string, env map[string]string) ServerConfig {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test executable: %v", err)
	}
	merged := map[string]string{helperServerEnv: "1"}
	for key, value := range env {
		merged[key] = value
	}
	return ServerConfig{
		Transport:  "stdio",
		Command:    executable,
		Args:       []string{"-test.run=^$"},
		Env:        merged,
		WorkingDir: workingDir,
	}
}

func writeServerConfig(t *testing.T, cfg ServerConfig) string {
	t.Helper()
	bytes, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	path := filepath.Join(t.TempDir(), "mcp.json")
	if err := os.WriteFile(path, bytes, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestStdioToolSetStartsServerWithEnvAndWorkingDir(t *testing.T) {
	workingDir := t.TempDir()
	secret := `a "quoted" $HOME value`
	toolSet := newStdioToolSet("helper", helperServerConfig(t, workingDir, map[string]string{"PROBE_SECRET": secret}), nil)
	t.Cleanup(func() { _ = toolSet.Close() })

	ctx := context.Background()
	if err := toolSet.Init(ctx); err != nil {
		t.Fatalf("init: %v", err)
	}

	var probe *stdioTool
	for _, current := range toolSet.Tools(ctx) {
		if current.Declaration().Name == "probe" {
			probe = current.(*stdioTool)
		}
	}
	if probe == nil {
		t.Fatal("expected probe tool to be listed")
	}
	if schema := probe.Declaration().InputSchema; schema == nil || schema.Properties["key"] == nil {
		t.Fatalf("expected input schema with key property, got %#v", schema)
	}

	result, err := probe.Call(ctx, []byte(`{"key":"PROBE_SECRET"}`))
	if err != nil {
		t.Fatalf("call probe: %v", err)
	}
	content := result.(*stdioToolResult).Content
	if len(content) != 1 {
		t.Fatalf("expected one content item, got %d", len(content))
	}
	text, ok := content[0].(mcpgo.TextContent)
	if !ok {
		t.Fatalf("expected text content, got %T", content[0])
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(text.Text), &got); err != nil {
		t.Fatalf("decode probe output %q: %v", text.Text, err)
	}
	if got["value"] != secret {
		t.Fatalf("expected env value %q, got %q", secret, got["value"])
	}
	wantDir, _ := filepath.EvalSymlinks(workingDir)
	gotDir, _ := filepath.EvalSymlinks(got["cwd"])
	if gotDir != wantDir {
		t.Fatalf("expected working dir %q, got %q", wantDir, gotDir)
	}
}

func TestBuildToolSetUsesNativeStdioAndAppliesToolFilter(t *testing.T) {
	configPath := writeServerConfig(t, helperServerConfig(t, "", nil))
	item := data_models.ExtensionItem{ID: "mcp:helper:default", Kind: "mcp", ConfigFilePath: configPath}
	manager := newTestManager(t)

	toolSet, err := manager.BuildToolSet(item, []string{mcpToolNamePrefix(item) + "probe"})
	if err != nil {
		t.Fatalf("build tool set: %v", err)
	}
	t.Cleanup(func() { _ = toolSet.Close() })
	if _, ok := toolSet.(*stdioToolSet); !ok {
		t.Fatalf("expected stdio servers to use the native stdio tool set, got %T", toolSet)
	}

	tools := toolSet.Tools(context.Background())
	if len(tools) != 1 || tools[0].Declaration().Name != "probe" {
		names := []string{}
		for _, current := range tools {
			names = append(names, current.Declaration().Name)
		}
		t.Fatalf("expected only the selected probe tool, got %v", names)
	}
}
