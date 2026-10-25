package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	agenttool "trpc.group/trpc-go/trpc-agent-go/tool"
	mcpgo "trpc.group/trpc-go/trpc-mcp-go"
)

// stdioClientInfo matches the client identity trpc-agent-go reports for remote transports.
var stdioClientInfo = mcpgo.Implementation{
	Name:    "trpc-agent-go",
	Version: "1.0.0",
}

// stdioToolSet runs a local stdio MCP server through trpc-mcp-go directly.
// trpc-agent-go's MCP ToolSet drops env/working_dir for stdio servers, so local
// servers are started here with native process settings instead of a shell wrapper.
type stdioToolSet struct {
	name    string
	config  mcpgo.StdioTransportConfig
	timeout time.Duration
	filter  agenttool.FilterFunc

	mu     sync.RWMutex
	client *mcpgo.StdioClient
	tools  []agenttool.Tool
}

func newStdioToolSet(name string, config ServerConfig, filter agenttool.FilterFunc) *stdioToolSet {
	transport := config.stdioTransportConfig()
	return &stdioToolSet{
		name:    name,
		config:  transport,
		timeout: transport.Timeout,
		filter:  filter,
	}
}

// Init starts the server and preloads its tools so callers can fail fast.
func (s *stdioToolSet) Init(ctx context.Context) error {
	if err := s.refresh(ctx); err != nil {
		return fmt.Errorf("failed to initialize MCP tool set %q: %w", s.name, err)
	}
	return nil
}

// Tools refreshes the tool list, falling back to the cached tools when the refresh fails.
func (s *stdioToolSet) Tools(ctx context.Context) []agenttool.Tool {
	_ = s.refresh(ctx)

	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]agenttool.Tool(nil), s.tools...)
}

// Close stops the server process.
func (s *stdioToolSet) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client == nil {
		return nil
	}
	err := s.client.Close()
	s.client = nil
	if err != nil {
		return fmt.Errorf("failed to close MCP session: %w", err)
	}
	return nil
}

func (s *stdioToolSet) Name() string {
	return s.name
}

func (s *stdioToolSet) refresh(ctx context.Context) error {
	client, err := s.session(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to MCP server: %w", err)
	}

	listCtx, cancel := s.withTimeout(ctx)
	defer cancel()
	resp, err := client.ListTools(listCtx, &mcpgo.ListToolsRequest{})
	if err != nil {
		return fmt.Errorf("failed to list tools from MCP server: %w", err)
	}

	tools := make([]agenttool.Tool, 0, len(resp.Tools))
	for _, current := range resp.Tools {
		tools = append(tools, newStdioTool(current, s))
	}
	if s.filter != nil {
		tools = agenttool.FilterTools(ctx, tools, s.filter)
	}

	s.mu.Lock()
	s.tools = tools
	s.mu.Unlock()
	return nil
}

// session returns the running client, starting and initializing the server on first use.
func (s *stdioToolSet) session(ctx context.Context) (*mcpgo.StdioClient, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client != nil {
		return s.client, nil
	}

	client, err := mcpgo.NewStdioClient(s.config, stdioClientInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP client: %w", err)
	}
	initCtx, cancel := s.withTimeout(ctx)
	defer cancel()
	if _, err := client.Initialize(initCtx, &mcpgo.InitializeRequest{}); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("failed to initialize MCP session: %w", err)
	}
	s.client = client
	return client, nil
}

func (s *stdioToolSet) callTool(ctx context.Context, name string, arguments map[string]any) (*mcpgo.CallToolResult, error) {
	client, err := s.session(ctx)
	if err != nil {
		return nil, err
	}

	callCtx, cancel := s.withTimeout(ctx)
	defer cancel()
	req := &mcpgo.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = arguments
	resp, err := client.CallTool(callCtx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to call tool %s: %w", name, err)
	}
	if len(resp.Content) == 0 && resp.StructuredContent != nil {
		structured, err := json.Marshal(resp.StructuredContent)
		if err != nil {
			return nil, fmt.Errorf("marshal structured content: %w", err)
		}
		return &mcpgo.CallToolResult{
			Result:            resp.Result,
			Content:           []mcpgo.Content{mcpgo.NewTextContent(string(structured))},
			StructuredContent: resp.StructuredContent,
			IsError:           resp.IsError,
		}, nil
	}
	return resp, nil
}

// withTimeout applies the configured timeout unless the caller already set a deadline.
func (s *stdioToolSet) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if s.timeout > 0 {
		if _, ok := ctx.Deadline(); !ok {
			return context.WithTimeout(ctx, s.timeout)
		}
	}
	return ctx, func() {}
}

// stdioTool exposes one MCP server tool to the agent.
type stdioTool struct {
	ref          mcpgo.Tool
	inputSchema  *agenttool.Schema
	outputSchema *agenttool.Schema
	toolSet      *stdioToolSet
}

func newStdioTool(ref mcpgo.Tool, toolSet *stdioToolSet) *stdioTool {
	current := &stdioTool{ref: ref, toolSet: toolSet}
	if ref.InputSchema != nil {
		current.inputSchema = convertMCPSchemaToSchema(ref.InputSchema)
	}
	if ref.OutputSchema != nil {
		current.outputSchema = convertMCPSchemaToSchema(ref.OutputSchema)
	}
	return current
}

func (t *stdioTool) Declaration() *agenttool.Declaration {
	return &agenttool.Declaration{
		Name:         t.ref.Name,
		Description:  t.ref.Description,
		InputSchema:  t.inputSchema,
		OutputSchema: t.outputSchema,
	}
}

func (t *stdioTool) Call(ctx context.Context, jsonArgs []byte) (any, error) {
	arguments := map[string]any{}
	if len(jsonArgs) > 0 {
		if err := json.Unmarshal(jsonArgs, &arguments); err != nil {
			return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
		}
	}
	result, err := t.toolSet.callTool(ctx, t.ref.Name, arguments)
	if err != nil {
		return nil, err
	}
	return &stdioToolResult{Content: result.Content, Meta: result.Meta, IsError: result.IsError}, nil
}

// stdioToolResult mirrors trpc-agent-go's MCP result shape: it marshals as the
// content slice while keeping metadata and the error flag available to callbacks.
type stdioToolResult struct {
	Content []mcpgo.Content
	Meta    map[string]any
	IsError bool
}

func (r *stdioToolResult) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Content)
}

func (r *stdioToolResult) GetMeta() map[string]any {
	return r.Meta
}

func (r *stdioToolResult) GetCallbackResult() any {
	return r.Content
}

func (r *stdioToolResult) RetryResultError() bool {
	return r.IsError
}
