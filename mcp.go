package streamsh

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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

// QuerySessionOutput is the JSON representation of query_session results.
type QuerySessionOutput struct {
	SessionID  string   `json:"session_id"`
	Title      string   `json:"title"`
	TotalLines int      `json:"total_lines"`
	Lines      []string `json:"lines"`
	NextCursor uint64   `json:"next_cursor,omitempty"`
	HasMore    bool     `json:"has_more"`
}

// RegisterMCPTools registers list_sessions and query_session on the MCP server.
func RegisterMCPTools(server *mcp.Server, store *Store) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_sessions",
		Description: "List all active and recent shell sessions being tracked by streamsh. Returns session IDs, titles, last commands, and connection status.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input ListSessionsInput) (*mcp.CallToolResult, any, error) {
		sessions := store.List()
		infos := make([]SessionInfo, len(sessions))
		for i, s := range sessions {
			infos[i] = SessionInfo{
				ID:          s.ShortID,
				Title:       s.Title,
				LastCommand: s.LastCommand,
				LineCount:   s.Buffer.Len(),
				CreatedAt:   s.CreatedAt.Format(time.RFC3339),
				Connected:   s.Connected,
			}
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
		sess, err := store.Resolve(input.Session)
		if err != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)},
				},
				IsError: true,
			}, nil, nil
		}

		output := QuerySessionOutput{
			SessionID:  sess.ShortID,
			Title:      sess.Title,
			TotalLines: sess.Buffer.Len(),
		}

		switch {
		case input.Search != "":
			maxResults := input.MaxResults
			if maxResults <= 0 {
				maxResults = 50
			}
			results := sess.Buffer.Search(input.Search, maxResults)
			output.Lines = make([]string, len(results))
			for i, r := range results {
				output.Lines[i] = fmt.Sprintf("[%d] %s", r.Seq, r.Line)
			}

		case input.LastN > 0:
			output.Lines = sess.Buffer.LastN(input.LastN)

		default:
			count := input.Count
			if count <= 0 {
				count = 100
			}
			lines, nextCursor, hasMore := sess.Buffer.ReadRange(input.Cursor, count)
			output.Lines = lines
			output.NextCursor = nextCursor
			output.HasMore = hasMore
		}

		result, _ := json.Marshal(output)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: string(result)},
			},
		}, nil, nil
	})
}

// NewMCPServer creates a configured MCP server with tools registered.
func NewMCPServer(store *Store) *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "streamsh",
			Version: "0.1.0",
		},
		nil,
	)
	RegisterMCPTools(server, store)
	return server
}
