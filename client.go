package streamsh

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/acarl005/stripansi"
	"github.com/creack/pty"
	"github.com/google/uuid"
	"golang.org/x/term"
)

// Client wraps a shell session in a PTY and streams output to the daemon.
type Client struct {
	Shell      string
	Title      string
	SocketPath string
	Logger     *slog.Logger
	Collab     bool

	conn      net.Conn
	enc       *json.Encoder
	scanner   *bufio.Scanner
	sessionID string
	shortID   string
	mu        sync.Mutex // protects conn, enc, scanner

	localBuf    *RingBuffer          // local ring buffer, always receives output
	connected   atomic.Bool          // whether currently connected to daemon
	lastCommand atomic.Pointer[string] // last detected command, for replay
	ptmx        *os.File             // PTY master, needed by reconnect for collab
	stopReconn  chan struct{}         // signals reconnection goroutine to stop
}

// Run starts the shell session and streams output to the daemon.
// It returns the shell's exit code.
func (c *Client) Run() (int, error) {
	// Check if already inside a streamsh session
	if id := os.Getenv("STREAMSH"); id != "" {
		fmt.Fprintf(os.Stderr, "Already in a streamsh session [%s]\n", id)
		return 1, nil
	}

	// Self-assign session identity
	c.sessionID = uuid.New().String()
	c.shortID = c.sessionID[:8]

	// Create local ring buffer
	c.localBuf = NewRingBuffer(10000)

	// Initialize reconnection control
	c.stopReconn = make(chan struct{})

	// Attempt initial connection (non-fatal if fails)
	if err := c.connect(); err != nil {
		c.Logger.Warn("could not connect to daemon, will retry in background", "err", err)
	}

	// Start background reconnection goroutine
	go c.reconnectionLoop()
	defer func() {
		close(c.stopReconn)
		c.disconnect()
	}()

	// Start shell in PTY
	shell := c.Shell
	if shell == "" {
		shell = os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
	}

	cmd := exec.Command(shell)
	streamshEnv := c.shortID
	if c.Title != "" {
		streamshEnv += " - " + c.Title
	}
	cmd.Env = append(os.Environ(), "STREAMSH="+streamshEnv)

	cleanup := c.setupShellPrompt(shell, cmd)
	defer cleanup()

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return 1, fmt.Errorf("starting pty: %w", err)
	}
	defer ptmx.Close()
	c.ptmx = ptmx

	// Handle terminal resize
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			pty.InheritSize(os.Stdin, ptmx)
		}
	}()
	ch <- syscall.SIGWINCH // initial size

	// Set stdin to raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return 1, fmt.Errorf("setting raw mode: %w", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	var wg sync.WaitGroup

	// stdin -> PTY (with command detection)
	go c.copyStdinToPTY(ptmx)

	// daemon -> PTY (collab mode: receive agent input)
	if c.Collab && c.connected.Load() {
		go c.handleIncomingMessages(ptmx)
	}

	// PTY -> stdout + daemon
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.copyPTYToStdout(ptmx)
	}()

	// Wait for shell to exit
	err = cmd.Wait()
	signal.Stop(ch)
	close(ch)

	// Close PTY to unblock copiers
	ptmx.Close()
	wg.Wait()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	return exitCode, nil
}

func (c *Client) connect() error {
	conn, err := net.Dial("unix", c.SocketPath)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.conn = conn
	c.enc = json.NewEncoder(conn)
	c.scanner = bufio.NewScanner(conn)
	c.scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	c.mu.Unlock()

	// Register session with self-assigned ID
	payload := mustMarshal(RegisterPayload{
		Title:     c.Title,
		Collab:    c.Collab,
		SessionID: c.sessionID,
	})
	c.sendMsg(Envelope{Type: MsgRegister, Payload: payload})

	// Read ack
	if c.scanner.Scan() {
		var env Envelope
		if err := json.Unmarshal(c.scanner.Bytes(), &env); err == nil && env.Type == MsgAck {
			var ack RegisterAck
			json.Unmarshal(env.Payload, &ack)
			c.Logger.Info("session registered", "id", ack.ShortID)
		}
	}

	c.connected.Store(true)

	// Replay local buffer to daemon
	c.replayBuffer()

	return nil
}

func (c *Client) disconnect() {
	if !c.connected.Load() {
		return
	}
	c.connected.Store(false)

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return
	}
	// Best-effort disconnect message
	c.enc.Encode(Envelope{Type: MsgDisconnect, SessionID: c.sessionID})
	c.conn.Close()
	c.conn = nil
	c.enc = nil
	c.scanner = nil
}

func (c *Client) replayBuffer() {
	lines := c.localBuf.AllLines()
	if len(lines) == 0 {
		return
	}

	const chunkSize = 500
	for i := 0; i < len(lines); i += chunkSize {
		end := i + chunkSize
		if end > len(lines) {
			end = len(lines)
		}
		chunk := lines[i:end]
		isLast := end >= len(lines)

		payload := ReplayPayload{Lines: chunk}
		if isLast {
			if cmd := c.getLastCommand(); cmd != "" {
				payload.LastCommand = cmd
			}
		}
		c.sendMsg(Envelope{
			Type:      MsgReplay,
			SessionID: c.sessionID,
			Payload:   mustMarshal(payload),
		})
	}
	c.Logger.Debug("replayed buffer to daemon", "lines", len(lines))
}

func (c *Client) reconnectionLoop() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopReconn:
			return
		case <-ticker.C:
			if c.connected.Load() {
				continue
			}

			// Clean up old connection if any
			c.mu.Lock()
			if c.conn != nil {
				c.conn.Close()
				c.conn = nil
				c.enc = nil
				c.scanner = nil
			}
			c.mu.Unlock()

			if err := c.connect(); err != nil {
				continue
			}
			c.Logger.Info("reconnected to daemon", "id", c.shortID)

			if c.Collab && c.ptmx != nil {
				go c.handleIncomingMessages(c.ptmx)
			}
		}
	}
}

func (c *Client) setLastCommand(cmd string) {
	c.lastCommand.Store(&cmd)
}

func (c *Client) getLastCommand() string {
	p := c.lastCommand.Load()
	if p == nil {
		return ""
	}
	return *p
}

func (c *Client) handleIncomingMessages(ptmx *os.File) {
	// Capture scanner reference locally to avoid race with reconnection
	c.mu.Lock()
	scanner := c.scanner
	c.mu.Unlock()

	if scanner == nil {
		return
	}

	for scanner.Scan() {
		var env Envelope
		if err := json.Unmarshal(scanner.Bytes(), &env); err != nil {
			c.Logger.Debug("failed to parse incoming message", "err", err)
			continue
		}
		if env.Type == MsgInput {
			var p InputPayload
			if env.Payload != nil {
				json.Unmarshal(env.Payload, &p)
			}
			if p.Text != "" {
				ptmx.Write([]byte(p.Text))
			}
		}
	}
	// Scanner ended — connection lost
	c.connected.Store(false)
}

func (c *Client) promptTag() string {
	if c.Title != "" {
		return fmt.Sprintf("[streamsh - %s (%s)]", c.Title, c.shortID)
	}
	return fmt.Sprintf("[streamsh - %s]", c.shortID)
}

func (c *Client) setupShellPrompt(shell string, cmd *exec.Cmd) (cleanup func()) {
	tag := c.promptTag()
	noop := func() {}

	if c.shortID == "" {
		return noop
	}

	base := filepath.Base(shell)

	switch {
	case base == "bash" || strings.HasPrefix(base, "bash"):
		dir, err := os.MkdirTemp("", "streamsh-rc-*")
		if err != nil {
			return noop
		}
		content := fmt.Sprintf(
			"[[ -f \"$HOME/.bashrc\" ]] && source \"$HOME/.bashrc\"\n"+
				"_STREAMSH_ORIG_PS1=\"$PS1\"\n"+
				"_STREAMSH_ORIG_PROMPT_COMMAND=\"$PROMPT_COMMAND\"\n"+
				"PROMPT_COMMAND='eval \"$_STREAMSH_ORIG_PROMPT_COMMAND\"; PS1=\"\\[\\e[35m\\]%s\\[\\e[0m\\] $_STREAMSH_ORIG_PS1\"'\n",
			tag,
		)
		rcPath := filepath.Join(dir, ".bashrc")
		if err := os.WriteFile(rcPath, []byte(content), 0644); err != nil {
			os.RemoveAll(dir)
			return noop
		}
		cmd.Args = []string{shell, "--rcfile", rcPath}
		return func() { os.RemoveAll(dir) }

	case base == "zsh" || strings.HasPrefix(base, "zsh"):
		dir, err := os.MkdirTemp("", "streamsh-rc-*")
		if err != nil {
			return noop
		}
		home := os.Getenv("HOME")
		escaped := strings.ReplaceAll(tag, "%", "%%")
		content := fmt.Sprintf(
			"[[ -f \"%s/.zshrc\" ]] && ZDOTDIR=\"%s\" source \"%s/.zshrc\"\n"+
				"_streamsh_orig_ps1=\"$PS1\"\n"+
				"_streamsh_precmd() { PS1=\"%%F{magenta}%s%%f $_streamsh_orig_ps1\" }\n"+
				"precmd_functions=(_streamsh_precmd $precmd_functions)\n",
			home, home, home, escaped,
		)
		rcPath := filepath.Join(dir, ".zshrc")
		if err := os.WriteFile(rcPath, []byte(content), 0644); err != nil {
			os.RemoveAll(dir)
			return noop
		}
		cmd.Env = append(cmd.Env, "ZDOTDIR="+dir)
		return func() { os.RemoveAll(dir) }

	case base == "fish" || strings.HasPrefix(base, "fish"):
		initScript := fmt.Sprintf(
			"functions -c fish_prompt _streamsh_orig_prompt\n"+
				"function fish_prompt\n"+
				"    set_color magenta\n"+
				"    echo -n '%s '\n"+
				"    set_color normal\n"+
				"    _streamsh_orig_prompt\n"+
				"end\n",
			tag,
		)
		cmd.Args = []string{shell, "-C", initScript}
		return noop

	default:
		// POSIX fallback
		dir, err := os.MkdirTemp("", "streamsh-rc-*")
		if err != nil {
			return noop
		}
		content := fmt.Sprintf("PS1='\\033[35m%s\\033[0m '$PS1\n", tag)
		rcPath := filepath.Join(dir, ".shrc")
		if err := os.WriteFile(rcPath, []byte(content), 0644); err != nil {
			os.RemoveAll(dir)
			return noop
		}
		cmd.Env = append(cmd.Env, "ENV="+rcPath)
		return func() { os.RemoveAll(dir) }
	}
}

func (c *Client) sendMsg(env Envelope) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return
	}
	if err := c.enc.Encode(env); err != nil {
		c.Logger.Debug("send error, marking disconnected", "err", err)
		c.connected.Store(false)
		c.conn.Close()
		c.conn = nil
		c.enc = nil
		c.scanner = nil
	}
}

func (c *Client) sendOutput(lines []string) {
	// Always write to local buffer, regardless of connection state
	for _, line := range lines {
		c.localBuf.Append(stripansi.Strip(line))
	}

	if !c.connected.Load() || len(lines) == 0 {
		return
	}
	c.sendMsg(Envelope{
		Type:      MsgOutput,
		SessionID: c.sessionID,
		Payload:   mustMarshal(OutputPayload{Lines: lines}),
	})
}

func (c *Client) sendCommand(cmd string) {
	if cmd == "" {
		return
	}
	c.setLastCommand(cmd)

	if !c.connected.Load() {
		return
	}
	c.sendMsg(Envelope{
		Type:      MsgCommand,
		SessionID: c.sessionID,
		Payload:   mustMarshal(CommandPayload{Command: cmd}),
	})
}

func (c *Client) copyStdinToPTY(ptmx *os.File) {
	var cmdBuf bytes.Buffer
	buf := make([]byte, 4096)

	for {
		n, err := os.Stdin.Read(buf)
		if n > 0 {
			ptmx.Write(buf[:n])

			// Detect commands: look for carriage return
			for _, b := range buf[:n] {
				if b == '\r' || b == '\n' {
					cmd := cmdBuf.String()
					cmdBuf.Reset()
					c.sendCommand(cmd)
				} else if b == 127 || b == '\b' {
					// Backspace: remove last byte from buffer
					if cmdBuf.Len() > 0 {
						cmdBuf.Truncate(cmdBuf.Len() - 1)
					}
				} else if b >= 32 { // printable
					cmdBuf.WriteByte(b)
				}
			}
		}
		if err != nil {
			return
		}
	}
}

func (c *Client) copyPTYToStdout(ptmx *os.File) {
	buf := make([]byte, 4096)
	var lineBuf bytes.Buffer
	var batch []string

	for {
		n, err := ptmx.Read(buf)
		if n > 0 {
			os.Stdout.Write(buf[:n])

			// Always assemble lines (local buffer + daemon if connected)
			for _, b := range buf[:n] {
				if b == '\n' {
					batch = append(batch, lineBuf.String())
					lineBuf.Reset()
				} else {
					lineBuf.WriteByte(b)
				}
			}
			if len(batch) > 0 {
				c.sendOutput(batch)
				batch = batch[:0]
			}
		}
		if err != nil {
			// Flush remaining line buffer
			if lineBuf.Len() > 0 {
				c.sendOutput([]string{lineBuf.String()})
			}
			if err != io.EOF {
				c.Logger.Debug("pty read error", "err", err)
			}
			return
		}
	}
}
