package streamsh

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"sync"
)

// DaemonClient connects to the daemon over a Unix socket and provides
// request-response methods for MCP tool operations.
type DaemonClient struct {
	conn    net.Conn
	enc     *json.Encoder
	scanner *bufio.Scanner
	mu      sync.Mutex // serializes request-response pairs
}

// NewDaemonClient dials the daemon Unix socket and returns a client.
func NewDaemonClient(socketPath string) (*DaemonClient, error) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("connecting to daemon: %w", err)
	}
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	return &DaemonClient{
		conn:    conn,
		enc:     json.NewEncoder(conn),
		scanner: scanner,
	}, nil
}

// Close closes the connection to the daemon.
func (dc *DaemonClient) Close() error {
	if dc.conn != nil {
		return dc.conn.Close()
	}
	return nil
}

// roundTrip sends a request and reads back a single response.
func (dc *DaemonClient) roundTrip(req Envelope) (Envelope, error) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if err := dc.enc.Encode(req); err != nil {
		return Envelope{}, fmt.Errorf("sending request: %w", err)
	}

	if !dc.scanner.Scan() {
		if err := dc.scanner.Err(); err != nil {
			return Envelope{}, fmt.Errorf("reading response: %w", err)
		}
		return Envelope{}, fmt.Errorf("connection closed")
	}

	var resp Envelope
	if err := json.Unmarshal(dc.scanner.Bytes(), &resp); err != nil {
		return Envelope{}, fmt.Errorf("parsing response: %w", err)
	}

	if resp.Type == MsgError {
		var ep ErrorPayload
		json.Unmarshal(resp.Payload, &ep)
		return Envelope{}, fmt.Errorf("%s", ep.Message)
	}

	return resp, nil
}

// ListSessions returns all sessions from the daemon.
func (dc *DaemonClient) ListSessions() ([]SessionInfo, error) {
	resp, err := dc.roundTrip(Envelope{Type: MsgListSessions})
	if err != nil {
		return nil, err
	}
	var result ListSessionsResponse
	if err := json.Unmarshal(resp.Payload, &result); err != nil {
		return nil, fmt.Errorf("parsing list response: %w", err)
	}
	return result.Sessions, nil
}

// QuerySession queries a specific session on the daemon.
func (dc *DaemonClient) QuerySession(p QuerySessionPayload) (*QuerySessionResponse, error) {
	resp, err := dc.roundTrip(Envelope{
		Type:    MsgQuerySession,
		Payload: mustMarshal(p),
	})
	if err != nil {
		return nil, err
	}
	var result QuerySessionResponse
	if err := json.Unmarshal(resp.Payload, &result); err != nil {
		return nil, fmt.Errorf("parsing query response: %w", err)
	}
	return &result, nil
}

// WriteSession sends input to a collaborative session via the daemon.
func (dc *DaemonClient) WriteSession(p WriteSessionPayload) (*WriteSessionResponse, error) {
	resp, err := dc.roundTrip(Envelope{
		Type:    MsgWriteSession,
		Payload: mustMarshal(p),
	})
	if err != nil {
		return nil, err
	}
	var result WriteSessionResponse
	if err := json.Unmarshal(resp.Payload, &result); err != nil {
		return nil, fmt.Errorf("parsing write response: %w", err)
	}
	return &result, nil
}
