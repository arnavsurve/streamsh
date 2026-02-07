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
		Description: "List all terminal sessions. Returns each session's ID, title, last command run, and connection status. Use this to find sessions relevant to your current task before querying their output.",
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
		Description: "Read output from a terminal session. Use last_n to get recent output (e.g. to check for errors after a change), search to find specific patterns in the output (e.g. error messages, stack traces), or cursor for paginated reading.",
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

// serverInstructions tells consuming agents when and how to use streamsh tools.
const serverInstructions = `You have access to the user's live terminal sessions via streamsh.

Use these tools proactively when you have a reason to:
- After making code changes, check sessions running the affected service or tests for errors or confirmation that the change worked.
- When the user mentions an error, unexpected behavior, or a failing build, check relevant sessions for logs and stack traces.
- When debugging, search session output for error messages, warnings, or relevant log lines.
- After the user runs a deploy, migration, or build, check the session to verify it succeeded.

Use list_sessions to see what's running (each session shows its last command), then query_session to read the output you need. Don't read sessions unless the output is relevant to what you're working on.`

// NewMCPServer creates a configured MCP server with tools registered.
func NewMCPServer(dc *DaemonClient) *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "streamsh",
			Version: "0.1.0",
		},
		&mcp.ServerOptions{
			Instructions: serverInstructions,
		},
	)
	RegisterMCPTools(server, dc)
	return server
}
