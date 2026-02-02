package streamsh

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/acarl005/stripansi"
	"github.com/google/uuid"
)

// Daemon manages the Unix socket listener and routes client connections.
type Daemon struct {
	Store      *Store
	BufferSize int
	Logger     *slog.Logger

	listener net.Listener
	wg       sync.WaitGroup
}

// DefaultSocketPath returns the default Unix socket path.
func DefaultSocketPath() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "streamsh.sock")
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("streamsh-%d", os.Getuid()), "streamsh.sock")
}

// Listen starts accepting connections on the Unix socket.
func (d *Daemon) Listen(ctx context.Context, socketPath string) error {
	// Clean up stale socket
	if _, err := os.Stat(socketPath); err == nil {
		conn, err := net.Dial("unix", socketPath)
		if err == nil {
			conn.Close()
			return fmt.Errorf("another daemon is already running on %s", socketPath)
		}
		os.Remove(socketPath)
	}

	// Ensure parent directory exists with restricted permissions
	dir := filepath.Dir(socketPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating socket directory: %w", err)
	}

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", socketPath, err)
	}
	d.listener = ln
	d.Logger.Info("listening", "path", socketPath)

	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				d.Logger.Error("accept error", "err", err)
				continue
			}
			d.wg.Add(1)
			go func() {
				defer d.wg.Done()
				d.handleConn(ctx, conn)
			}()
		}
	}()

	return nil
}

// Close shuts down the listener and waits for connections to finish.
func (d *Daemon) Close() {
	if d.listener != nil {
		d.listener.Close()
	}
	d.wg.Wait()
}

func (d *Daemon) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	enc := json.NewEncoder(conn)

	var sessionID uuid.UUID

	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}

		var env Envelope
		if err := json.Unmarshal(scanner.Bytes(), &env); err != nil {
			d.Logger.Error("bad message", "err", err)
			continue
		}

		switch env.Type {
		case MsgRegister:
			var p RegisterPayload
			if env.Payload != nil {
				json.Unmarshal(env.Payload, &p)
			}
			bufSize := d.BufferSize
			if p.BufferSize > 0 {
				bufSize = p.BufferSize
			}
			sess := d.Store.Create(p.Title, bufSize)
			sessionID = sess.ID
			d.Logger.Info("session registered", "id", sess.ShortID, "title", p.Title)

			enc.Encode(Envelope{
				Type: MsgAck,
				Payload: mustMarshal(RegisterAck{
					SessionID: sess.ID.String(),
					ShortID:   sess.ShortID,
				}),
			})

		case MsgOutput:
			var p OutputPayload
			if env.Payload != nil {
				json.Unmarshal(env.Payload, &p)
			}
			sess, ok := d.Store.Get(sessionID)
			if !ok {
				continue
			}
			for _, line := range p.Lines {
				sess.Buffer.Append(stripansi.Strip(line))
			}
			sess.LastActivity = time.Now()

		case MsgCommand:
			var p CommandPayload
			if env.Payload != nil {
				json.Unmarshal(env.Payload, &p)
			}
			sess, ok := d.Store.Get(sessionID)
			if !ok {
				continue
			}
			sess.LastCommand = p.Command
			sess.LastActivity = time.Now()

		case MsgDisconnect:
			sess, ok := d.Store.Get(sessionID)
			if ok {
				sess.Connected = false
				sess.LastActivity = time.Now()
				d.Logger.Info("session disconnected", "id", sess.ShortID)
			}
			return
		}
	}

	// Connection closed without disconnect message
	if sess, ok := d.Store.Get(sessionID); ok {
		sess.Connected = false
		sess.LastActivity = time.Now()
	}
}

// SocketPathFromEnv returns the socket path from the STREAMSH_SOCKET env var,
// or the default path.
func SocketPathFromEnv() string {
	if p := os.Getenv("STREAMSH_SOCKET"); p != "" {
		return p
	}
	return DefaultSocketPath()
}

func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// GetUid returns the current user's UID as used in temp paths.
// Exported for testing convenience.
func GetUid() string {
	return strconv.Itoa(os.Getuid())
}

