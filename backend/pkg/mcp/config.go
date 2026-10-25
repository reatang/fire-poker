package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	toolmcp "trpc.group/trpc-go/trpc-agent-go/tool/mcp"
	mcpgo "trpc.group/trpc-go/trpc-mcp-go"
)

// ServerConfig is the persisted MCP server connection config used by this app.
type ServerConfig struct {
	Transport   string            `json:"transport"`
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Timeout     time.Duration     `json:"timeout,omitempty"`
	WorkingDir  string            `json:"working_dir,omitempty"`
	ServerURL   string            `json:"server_url,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Description string            `json:"description,omitempty"`
}

const defaultStdioTimeout = 30 * time.Second

type rawServerListConfig struct {
	MCPServers map[string]ServerConfig `json:"mcpServers"`
}

// loadServerConfig reads a JSON MCP config file.
func loadServerConfig(configPath string) (ServerConfig, error) {
	bytes, err := os.ReadFile(configPath)
	if err != nil {
		return ServerConfig{}, err
	}

	cfg := ServerConfig{}
	if err := json.Unmarshal(bytes, &cfg); err != nil {
		return ServerConfig{}, err
	}

	if cfg.Command != "" || cfg.ServerURL != "" || cfg.Transport != "" {
		if cfg.Transport == "" {
			cfg.Transport = "stdio"
		}
		return cfg, nil
	}

	rawList := rawServerListConfig{}
	if err := json.Unmarshal(bytes, &rawList); err != nil {
		return ServerConfig{}, err
	}
	for _, server := range rawList.MCPServers {
		if server.Transport == "" {
			server.Transport = "stdio"
		}
		return server, nil
	}

	return ServerConfig{}, fmt.Errorf("invalid mcp config: no server definition found")
}

// stdioTransportConfig converts a local stdio server config into trpc-mcp-go process
// parameters. Env and working directory are applied to the process directly, so no
// shell is involved and the same config works on Windows.
func (c ServerConfig) stdioTransportConfig() mcpgo.StdioTransportConfig {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = defaultStdioTimeout
	}

	var env map[string]string
	if len(c.Env) > 0 {
		env = make(map[string]string, len(c.Env))
		for key, value := range c.Env {
			env[key] = value
		}
	}

	return mcpgo.StdioTransportConfig{
		ServerParams: mcpgo.StdioServerParameters{
			Command:    c.Command,
			Args:       append([]string(nil), c.Args...),
			Env:        env,
			WorkingDir: c.WorkingDir,
		},
		Timeout: timeout,
	}
}

// toToolSetConfig converts a remote (SSE/streamable) server config to the trpc-agent-go MCP config.
func (c ServerConfig) toToolSetConfig() toolmcp.ConnectionConfig {
	return toolmcp.ConnectionConfig{
		Transport:   c.Transport,
		Timeout:     c.Timeout,
		ServerURL:   c.ServerURL,
		Headers:     c.Headers,
		Description: c.Description,
	}
}
