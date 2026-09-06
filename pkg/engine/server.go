package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

// hosts a file manifest and serves chunk byte ranges to peers
type TransferServer struct {
	Manifest *FileManifest
	FilePath string
	Port     int

	server *http.Server
	wg     sync.WaitGroup
}

// initializes a new server instance with a calculated manifest
func NewTransferServer(filePath string, port int) (*TransferServer, error) {
	manifest, err := GenerateManifest(filePath)
	if err != nil {
		return nil, fmt.Errorf("Failed to generate manifest for server: %w", err)
	}

	return &TransferServer{
		Manifest: manifest,
		FilePath: filePath,
		Port:     port,
	}, nil
}

// start launches the HTTP server in the background using goroutine
func (ts *TransferServer) Start() error {
	mux := http.NewServeMux()

	// endpoints 1: receiever calls this first to get the blueprint (file metadata & hashes)
	mux.HandleFunc("/manifest", ts.handleManifest)

	// endpoints 2: receiever calls this concurrently with range headers to fetch chunks
	mux.HandleFunc("/download", ts.handleDownload)

	ts.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", ts.Port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 60 * time.Second, // allows time for big chunks on slower wifi or connection
	}

	// create TCP listener explicitly, allowing to verify the port bound successfully
	listener, err := net.Listen("tcp", ts.server.Addr)
	if err != nil {
		return fmt.Errorf("failed to bind port &d: %w", ts.Port, err)
	}

	ts.wg.Add(1)
	go func() {
		defer ts.wg.Done()
		if err := ts.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Server error: %v\n", err)
		}
	}()

	return nil
}

// stop gracefully shuts down the HTTP server
func (ts *TransferServer) Stop() error {
	if ts.server == nil {
		return nil
	}

	// give active chunk downloads 5 seconds to finish before forcing shut down
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ts.server.Shutdown(ctx)
	ts.wg.Wait()

	return err
}

// serves the FileManifest JSON blueprint to rwquesting receivers
func (ts *TransferServer) handleManifest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ts.Manifest)
}

// handles HTTP range Requests to serve specific file byte chunks
func (ts *TransferServer) handleDownload(w http.ResponseWriter, r *http.Request) {
	file, err := os.Open(ts.FilePath)
	if err != nil {
		http.Error(w, "File not found on host", http.StatusNotFound)
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		http.Error(w, "Unable to read file info", http.StatusInternalServerError)
		return
	}

	// set headers to receiver knows filename and binary format
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", ts.Manifest.FileName))
	w.Header().Set("Accept-Ranges", "bytes") // signal to receiver that we support range chunking

	// automatically parses "Range: bytes=X-Y" from request headers,
	// seeks to offset X, reads Y	bytes, and responds with HTTP 206 Partial Content
	http.ServeContent(w, r, fileInfo.Name(), fileInfo.ModTime(), file)
}
