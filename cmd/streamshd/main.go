package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/arnavsurve/streamsh"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	socketPath := flag.String("socket", streamsh.SocketPathFromEnv(), "Unix socket path")
	bufferSize := flag.Int("buffer-size", 10000, "Lines per session ring buffer")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	var level slog.Level
	switch *logLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		logger.Info("shutting down")
		cancel()
	}()

	// Try to start daemon — non-fatal if one is already running
	daemon := &streamsh.Daemon{
		Store:      streamsh.NewStore(),
		BufferSize: *bufferSize,
		Logger:     logger,
	}
	err := daemon.Listen(ctx, *socketPath)
	if err != nil && !errors.Is(err, streamsh.ErrDaemonAlreadyRunning) {
		logger.Error("failed to start daemon", "err", err)
		os.Exit(1)
	}
	daemonOwner := err == nil

	if daemonOwner {
		defer func() {
			daemon.Close()
			os.Remove(*socketPath)
		}()
	} else {
		logger.Info("daemon already running, connecting as MCP proxy")
	}

	// Connect to daemon for MCP operations
	dc, err := streamsh.NewDaemonClient(*socketPath)
	if err != nil {
		logger.Error("failed to connect to daemon", "err", err)
		os.Exit(1)
	}
	defer dc.Close()

	// Run MCP server on stdio using DaemonClient
	server := streamsh.NewMCPServer(dc)
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		if ctx.Err() == nil {
			logger.Error("mcp server error", "err", err)
			os.Exit(1)
		}
	}
}
