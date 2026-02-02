package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/arnavsurve/streamsh"
)

func main() {
	socketPath := flag.String("socket", streamsh.SocketPathFromEnv(), "Unix socket path")
	title := flag.String("title", "", "Session title (auto-generated if empty)")
	shell := flag.String("shell", "", "Shell to launch (defaults to $SHELL)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	client := &streamsh.Client{
		Shell:      *shell,
		Title:      *title,
		SocketPath: *socketPath,
		Logger:     logger,
	}

	exitCode, err := client.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "streamsh: %v\n", err)
		os.Exit(1)
	}
	os.Exit(exitCode)
}
