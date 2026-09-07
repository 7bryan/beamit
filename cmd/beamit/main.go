package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/7bryan/beamit/pkg/discovery"
	"github.com/7bryan/beamit/pkg/engine"
)

func main() {
	fmt.Println("BeamIt: ")
	if len(os.Args) < 2 {
		fmt.Println("Usage:")
		fmt.Println("		go run ./cmd/beamit send <filepath>")
		fmt.Println("		go run ./cmd/beamit receive") // removing manual URl
		return
	}

	mode := os.Args[1]

	switch mode {
	case "send":
		if len(os.Args) < 3 {
			fmt.Println("Usage: go run ./cmd/beamit send <filepath>")
			return
		}
		runServer(os.Args[2])

	case "receive":
		runClient()
	}
}

func runServer(filePath string) {
	port := 8080
	server, err := engine.NewTransferServer(filePath, port)
	if err != nil {
		fmt.Printf("Error initializing server: %v\n", err)
		return
	}

	if err := server.Start(); err != nil {
		fmt.Printf("Error starting server: %v\n", err)
		return
	}

	// start mDNS announcement on local network
	mdnsServer, err := discovery.AnnouncePeer(port, server.Manifest.FileName)
	if err != nil {
		fmt.Printf("Warning: mDNS auto-discovery failed: %v\n", err)
	} else {
		defer mdnsServer.Shutdown()
		fmt.Println("Broadcasting presence on local Wi-Fi via mDNS...")
	}

	fmt.Printf("Serving '%s' on http://localhost:%d\n", server.Manifest.FileName, port)
	fmt.Println("Press Ctrl+C")

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)
	<-stopChan

	server.Stop()
	fmt.Println("Server stopped")
}

func runClient() {
	fmt.Println("Scanning local Wi-Fi network for active BeamIt senders...")

	peers, err := discovery.DiscoverPeers(3 * time.Second)
	if err != nil {
		fmt.Printf("Discovery error: %v\n", err)
		return
	}

	if len(peers) == 0 {
		fmt.Println("No active BeamIt senders found on local network")
		return
	}

	// automatically pick the first discovered peer (for now)
	targetPeer := peers[0]
	serverUrl := fmt.Sprintf("http://%s:%d", targetPeer.IP, targetPeer.Port)

	fmt.Printf("Found peer '%s' at %s\n", targetPeer.ID, serverUrl)

	client := engine.NewTransferClient(serverUrl, 4) // 4 concurent worker

	fmt.Printf("Connecting to %s...\n", serverUrl)
	manifest, err := client.FetchManifest()
	if err != nil {
		fmt.Printf("Error fetching manifest: %v\n", err)
		return
	}

	fmt.Printf("Downloading '%s' (%d bytes, %d chunks)\n", manifest.FileName, manifest.FileSize, manifest.TotalChunks)

	err = client.DownloadFile(manifest, "./")
	if err != nil {
		fmt.Printf("Download failed: %v\n", err)
		return
	}

	fmt.Println("Download completed successfully & verified via SHA-256")
}
