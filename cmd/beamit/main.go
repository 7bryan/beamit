package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/7bryan/beamit/pkg/engine"
)

func main() {
	fmt.Println("BeamIt: ")
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run ./cmd/beamit <path-to-test-file>")
		return
	}

	filePath := os.Args[1]
	port := 8000

	fmt.Printf("Initializing BeamIt server for: %s\n", filePath)
	server, err := engine.NewTransferServer(filePath, port)
	if err != nil {
		fmt.Printf("Error creating server: %v\n", err)
	}

	err = server.Start()
	if err != nil {
		fmt.Printf("Error starting server: %v\n", err)
		return
	}

	fmt.Printf("Server running on http://localhost:%d\n", port)
	fmt.Println("Endpoints available:")
	fmt.Printf("	- Manifest: http://localhost:%d/manifest\n", port)
	fmt.Printf("	- Download: http://localhost:%d/download\n", port)
	fmt.Println("\nPress Ctrl+C to stop server")

	// wait to interupt signal
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)
	<-stopChan

	fmt.Println("\nShutting down server")
	server.Stop()
	fmt.Println("Server Stopped gracefully")
}
