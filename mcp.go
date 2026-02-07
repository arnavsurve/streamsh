package streamsh

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SessionInfo is the JSON representation of a session in list_sessions output.
type SessionInfo struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	LastCommand string `json:"last_command"`
	LineCount   int    `json:"line_count"`
	CreatedAt   string `json:"created_at"`
	Connected   bool   `json:"connected"`
	Collab      bool   `json:"collab"`
}

// ListSessionsInput is the input for the list_sessions tool.
type ListSessionsInput struct{}

// QuerySessionInput is the input for the query_session tool.
type QuerySessionInput struct {
	Session    string `json:"session" jsonschema:"required,Session identifier: short ID, UUID, or title"`
	Search     string `json:"search,omitempty" jsonschema:"Fuzzy/substring search pattern to match against output lines"`
	LastN      int    `json:"last_n,omitempty" jsonschema:"Return the last N lines of output"`
	Cursor     uint64 `json:"cursor,omitempty" jsonschema:"Start reading from this sequence number for pagination"`
	Count      int    `json:"count,omitempty" jsonschema:"Number of lines to return with cursor mode (default 100)"`
	MaxResults int    `json:"max_results,omitempty" jsonschema:"Max results for search mode (default 50)"`
}

// WriteSessionInput is the input for the write_session tool.
type WriteSessionInput struct {
	Session string `json:"session" jsonschema:"required,Session identifier: short ID, UUID, or title"`
	Text    string `json:"text" jsonschema:"required,Raw text to write to the session PTY. Text is written byte-for-byte to the PTY. To press Enter/execute a command you MUST include an actual newline character at the end of your text (not a literal backslash-n). Only works on collaborative sessions (started with --collab)."`
}

// RegisterMCPTools registers list_sessions, query_session, and write_session on the MCP server.
func RegisterMCPTools(server *mcp.Server, dc *DaemonClient) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_sessions",
		Description: "List all active and recent shell sessions being tracked by streamsh. Returns session IDs, titles, last commands, and connection status.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ListSessionsInput) (*mcp.CallToolResult, any, error) {
		infos, err := dc.ListSessions()
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)},
				},
				IsError: true,
			}, nil, nil
		}

		result, _ := json.Marshal(map[string]any{"sessions": infos})
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: string(result)},
			},
		}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "query_session",
		Description: "Query shell output from a specific session. Supports fuzzy search, reading the last N lines, or cursor-based pagination through the output buffer.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input QuerySessionInput) (*mcp.CallToolResult, any, error) {
		resp, err := dc.QuerySession(QuerySessionPayload{
			Session:    input.Session,
			Search:     input.Search,
			LastN:      input.LastN,
			Cursor:     input.Cursor,
			Count:      input.Count,
			MaxResults: input.MaxResults,
		})
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)},
				},
				IsError: true,
			}, nil, nil
		}

		result, _ := json.Marshal(resp)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: string(result)},
			},
		}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "write_session",
		Description: "Send raw text input to a collaborative shell session's PTY. Text is written byte-for-byte — to press Enter and execute a command, include an actual newline character at the end of your text (not a literal backslash-n). Only works on sessions started with the --collab flag. The user sees all input in real-time.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input WriteSessionInput) (*mcp.CallToolResult, any, error) {
		resp, err := dc.WriteSession(WriteSessionPayload{
			Session: input.Session,
			Text:    input.Text,
		})
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)},
				},
				IsError: true,
			}, nil, nil
		}

		result, _ := json.Marshal(resp)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: string(result)},
			},
		}, nil, nil
	})
}

// NewMCPServer creates a configured MCP server with tools registered.
func NewMCPServer(dc *DaemonClient) *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "streamsh",
			Version: "0.1.0",
		},
		nil,
	)
	RegisterMCPTools(server, dc)
	return server
}
