package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// hosts a file manifest and serves chunk byte ranges to peers
type TransferServer struct {
	Manifest *FileManifest
	FilePath string
	Port     int

	mu     sync.RWMutex // guards Manifest and filePath, mutable after start()
	server *http.Server
	wg     sync.WaitGroup
}

func NewTransferServer(port int) *TransferServer {
	return &TransferServer{Port: port}
}

func (ts *TransferServer) Share(filePath string) error {
	manifest, err := GenerateManifest(filePath)
	if err != nil {
		return fmt.Errorf("Failed to generate Manifest: %w", err)
	}

	ts.mu.Lock()
	ts.FilePath = filePath
	ts.Manifest = manifest
	ts.mu.Unlock()

	return nil
}

// start launches the HTTP server in the background using goroutine
func (ts *TransferServer) Start() error {
	mux := http.NewServeMux()

	// endpoints 1: receiever calls this first to get the blueprint (file metadata & hashes)
	mux.HandleFunc("/manifest", ts.handleManifest)

	// endpoints 2: receiever calls this concurrently with range headers to fetch chunks
	mux.HandleFunc("/download", ts.handleDownload)

	// endpoints 3: share
	mux.HandleFunc("/share", ts.handleShare)

	ts.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", ts.Port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 60 * time.Second, // allows time for big chunks on slower wifi or connection
	}

	// create TCP listener explicitly, allowing to verify the port bound successfully
	listener, err := net.Listen("tcp", ts.server.Addr)
	if err != nil {
		return fmt.Errorf("failed to bind port %d: %w", ts.Port, err)
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
	ts.mu.RLock()
	manifest := ts.Manifest
	ts.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if manifest == nil {
		w.WriteHeader(http.StatusNoContent) // nothing shared yet
		return
	}
	json.NewEncoder(w).Encode(ts.Manifest)
}

// handles HTTP range Requests to serve specific file byte chunks
func (ts *TransferServer) handleDownload(w http.ResponseWriter, r *http.Request) {
	ts.mu.RLock()
	filePath := ts.FilePath
	manifest := ts.Manifest
	ts.mu.RUnlock()

	if manifest == nil {
		http.Error(w, "Nothing is currently shared", http.StatusNotFound)
		return
	}

	file, err := os.Open(filePath)
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

const maxUploadSize = 10 << 30 // 10 GB cap on shared file,
const sharedFileDir = "./shared"

func (ts *TransferServer) handleShare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB in-memory buffer, rest spills to temp files
		http.Error(w, "File too large or malformed upload", http.StatusBadRequest)
		return
	}

	uploadedFile, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Missing 'file' field in upload", http.StatusBadRequest)
		return
	}
	defer uploadedFile.Close()

	if err := os.MkdirAll(sharedFileDir, 0755); err != nil {
		http.Error(w, "Failed to prepare storage", http.StatusInternalServerError)
		return
	}

	destPath := filepath.Join(sharedFileDir, filepath.Base(header.Filename))
	destFile, err := os.Create(destPath)
	if err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, uploadedFile); err != nil {
		http.Error(w, "Failed to write file", http.StatusInternalServerError)
		return
	}

	if err := ts.Share(destPath); err != nil {
		http.Error(w, "Failed to process shared file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Sharing '%s'\n", filepath.Base(destPath))
}
