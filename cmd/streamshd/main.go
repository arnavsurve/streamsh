package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/asurve/streamsh"
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

	store := streamsh.NewStore()

	// Start daemon (Unix socket listener)
	daemon := &streamsh.Daemon{
		Store:      store,
		BufferSize: *bufferSize,
		Logger:     logger,
	}
	if err := daemon.Listen(ctx, *socketPath); err != nil {
		logger.Error("failed to start daemon", "err", err)
		os.Exit(1)
	}
	defer func() {
		daemon.Close()
		os.Remove(*socketPath)
	}()

	// Run MCP server on stdio
	server := streamsh.NewMCPServer(store)
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		if ctx.Err() == nil {
			logger.Error("mcp server error", "err", err)
			os.Exit(1)
		}
	}
}
