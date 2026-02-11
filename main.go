package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/patrickjaja/claude-cowork-service/native"
	"github.com/patrickjaja/claude-cowork-service/pipe"
)

var version = "dev"

func main() {
	socketPath := flag.String("socket", defaultSocketPath(), "Unix socket path")
	debug := flag.Bool("debug", false, "Enable debug logging")
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("cowork-svc-linux %s\n", version)
		os.Exit(0)
	}

	if *debug {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	} else {
		log.SetFlags(log.LstdFlags)
	}

	log.Printf("cowork-svc-linux %s starting (native backend)", version)
	log.Printf("Socket: %s", *socketPath)

	// Create native backend (executes directly on host, no VM)
	backend := native.NewBackend(*debug)

	// Create and start the Unix socket server
	server := pipe.NewServer(*socketPath, backend, *debug)
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	log.Printf("Listening on %s", *socketPath)

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("Received %s, shutting down...", sig)
	backend.Shutdown()
}

func defaultSocketPath() string {
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		return filepath.Join(xdg, "cowork-vm-service.sock")
	}
	return "/tmp/cowork-vm-service.sock"
}

