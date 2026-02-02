package streamsh

import "encoding/json"

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
)

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
