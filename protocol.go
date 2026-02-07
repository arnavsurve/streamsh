package streamsh

import (
	"encoding/json"
	"errors"
)

// MsgType identifies the kind of message sent over the Unix socket.
type MsgType string

const (
	MsgRegister   MsgType = "register"
	MsgOutput     MsgType = "output"
	MsgCommand    MsgType = "command"
	MsgDisconnect MsgType = "disconnect"
	MsgInput      MsgType = "input"
	MsgAck        MsgType = "ack"
	MsgError      MsgType = "error"

	MsgReplay MsgType = "replay" // historical buffer replay on reconnect

	// MCP-proxy request types (MCP server → daemon)
	MsgListSessions MsgType = "list_sessions"
	MsgQuerySession MsgType = "query_session"
	MsgWriteSession MsgType = "write_session"
)

// ErrDaemonAlreadyRunning is returned by Daemon.Listen when another daemon
// is already listening on the socket.
var ErrDaemonAlreadyRunning = errors.New("daemon already running")

// Envelope is the wire format for all IPC messages (newline-delimited JSON).
type Envelope struct {
	Type      MsgType         `json:"type"`
	SessionID string          `json:"session_id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

// RegisterPayload is sent by the client to create a new session.
type RegisterPayload struct {
	Title      string `json:"title,omitempty"`
	BufferSize int    `json:"buffer_size,omitempty"`
	Collab     bool   `json:"collab,omitempty"`
	SessionID  string `json:"session_id,omitempty"` // client-assigned UUID for reconnection
}

// RegisterAck is sent by the daemon after a successful registration.
type RegisterAck struct {
	SessionID string `json:"session_id"`
	ShortID   string `json:"short_id"`
}

// OutputPayload carries shell output lines from client to daemon.
type OutputPayload struct {
	Lines []string `json:"lines"`
}

// CommandPayload carries the last detected command from client to daemon.
type CommandPayload struct {
	Command string `json:"command"`
}

// InputPayload carries text from daemon to client to be written to the PTY.
type InputPayload struct {
	Text string `json:"text"`
}

// ErrorPayload carries an error message from daemon to client.
type ErrorPayload struct {
	Message string `json:"message"`
}

// ReplayPayload carries historical buffer content on reconnect.
type ReplayPayload struct {
	Lines       []string `json:"lines"`
	LastCommand string   `json:"last_command,omitempty"`
}

// ListSessionsResponse is the daemon response for MsgListSessions.
type ListSessionsResponse struct {
	Sessions []SessionInfo `json:"sessions"`
}

// QuerySessionPayload is the request payload for MsgQuerySession.
type QuerySessionPayload struct {
	Session    string `json:"session"`
	Search     string `json:"search,omitempty"`
	LastN      int    `json:"last_n,omitempty"`
	Cursor     uint64 `json:"cursor,omitempty"`
	Count      int    `json:"count,omitempty"`
	MaxResults int    `json:"max_results,omitempty"`
}

// QuerySessionResponse is the daemon response for MsgQuerySession.
type QuerySessionResponse struct {
	SessionID  string   `json:"session_id"`
	Title      string   `json:"title"`
	TotalLines int      `json:"total_lines"`
	Lines      []string `json:"lines"`
	NextCursor uint64   `json:"next_cursor,omitempty"`
	HasMore    bool     `json:"has_more"`
}

// WriteSessionPayload is the request payload for MsgWriteSession.
type WriteSessionPayload struct {
	Session string `json:"session"`
	Text    string `json:"text"`
}

// WriteSessionResponse is the daemon response for MsgWriteSession.
type WriteSessionResponse struct {
	Success   bool   `json:"success"`
	SessionID string `json:"session_id"`
	BytesSent int    `json:"bytes_sent"`
}
