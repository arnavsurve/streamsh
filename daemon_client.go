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
	socketPath string
	conn       net.Conn
	enc        *json.Encoder
	scanner    *bufio.Scanner
	mu         sync.Mutex // serializes request-response pairs
}

// NewDaemonClient dials the daemon Unix socket and returns a client.
func NewDaemonClient(socketPath string) (*DaemonClient, error) {
	dc := &DaemonClient{socketPath: socketPath}
	if err := dc.dial(); err != nil {
		return nil, err
	}
	return dc, nil
}

// dial connects (or reconnects) to the daemon socket.
func (dc *DaemonClient) dial() error {
	if dc.conn != nil {
		dc.conn.Close()
	}
	conn, err := net.Dial("unix", dc.socketPath)
	if err != nil {
		dc.conn = nil
		dc.enc = nil
		dc.scanner = nil
		return fmt.Errorf("connecting to daemon: %w", err)
	}
	dc.conn = conn
	dc.enc = json.NewEncoder(conn)
	dc.scanner = bufio.NewScanner(conn)
	dc.scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	return nil
}

// Close closes the connection to the daemon.
func (dc *DaemonClient) Close() error {
	if dc.conn != nil {
		return dc.conn.Close()
	}
	return nil
}

// roundTrip sends a request and reads back a single response.
// On connection failure, it reconnects and retries once.
func (dc *DaemonClient) roundTrip(req Envelope) (Envelope, error) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	resp, err := dc.doRoundTrip(req)
	if err != nil {
		// Connection may be stale — reconnect and retry once
		if dialErr := dc.dial(); dialErr != nil {
			return Envelope{}, fmt.Errorf("reconnect failed: %w (original: %w)", dialErr, err)
		}
		return dc.doRoundTrip(req)
	}
	return resp, nil
}

// doRoundTrip performs a single send+receive without reconnection.
func (dc *DaemonClient) doRoundTrip(req Envelope) (Envelope, error) {
	if dc.enc == nil {
		return Envelope{}, fmt.Errorf("not connected")
	}

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
