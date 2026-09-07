package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sync"
	"time"
)

// manages the concurrent chunk download process
type TransferClient struct {
	ServerUrl   string
	WorkerCount int
	HttpClient  *http.Client
}

// initializes a client with a custom HTTP connection pool
func NewTransferClient(serverUrl string, workerCount int) *TransferClient {
	return &TransferClient{
		ServerUrl:   serverUrl,
		WorkerCount: workerCount,
		HttpClient: &http.Client{
			Transport: &http.Transport{
				MaxIdleConnsPerHost: workerCount, // allow reuseable TCP sockets for all workers
			},
			Timeout: 30 * time.Second,
		},
	}
}

// retrieves the FileManifest from the host server
func (c *TransferClient) FetchManifest() (*FileManifest, error) {
	resp, err := c.HttpClient.Get(fmt.Sprintf("%s/manifest", c.ServerUrl))
	if err != nil {
		return nil, fmt.Errorf("failed to reach server manifest: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned non-200 status: %d", resp.StatusCode)
	}

	var manifest FileManifest
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest JSON: %w", err)
	}

	return &manifest, nil
}

// coordinates worker goroutines to fetch, verify, and write all chunks
func (c *TransferClient) DownloadFile(manifest *FileManifest, outputDir string) error {
	outputPath := filepath.Join(outputDir, manifest.FileName)

	// pre-allocate empty file space on disk matching total file size
	if err := PreallocateFile(outputPath, manifest.FileSize); err != nil {
		return fmt.Errorf("failed pre-allocating file space: %w", err)
	}

	// set up channels and WaitGroup for worker pool
	chunkQueue := make(chan ChunkMetadata, len(manifest.Chunks))
	errChan := make(chan error, len(manifest.Chunks))
	var wg sync.WaitGroup

	// spawn concurrent worker goroutines
	for i := 0; i < c.WorkerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for chunk := range chunkQueue {
				if err := c.downloadAndWriteChunk(chunk, outputPath); err != nil {
					errChan <- fmt.Errorf("worker %d failed chunk %d: %w", workerID, chunk.ID, err)
					return
				}
			}
		}(i)
	}

	// push all chunk metadata jobs into the queue channel
	for _, chunk := range manifest.Chunks {
		chunkQueue <- chunk
	}
	close(chunkQueue) // signal worker that no more chunks are coming

	// wait for workers to finish
	wg.Wait()
	close(errChan)

	// check if any worker reported errors
	if len(errChan) > 0 {
		return <-errChan // return first encountered error
	}

	return nil
}

// fetches a single byte range, verifies SHA-256, and writes at offset
func (c *TransferClient) downloadAndWriteChunk(chunk ChunkMetadata, outputPath string) error {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/download", c.ServerUrl), nil)
	if err != nil {
		return err
	}

	// set HTTP Range Header (e.g., Range: bytes=1048576-2097151)
	endOffset := chunk.Offset + chunk.Size - 1
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", chunk.Offset, endOffset))

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// read raw body bytes into memory buffer
	chunkData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed reading response body: %w", err)
	}

	// verify cryptographic hash integrity before writing to disk
	hasher := sha256.New()
	hasher.Write(chunkData)
	calculatedHash := hex.EncodeToString(hasher.Sum(nil))

	if calculatedHash != chunk.Hash {
		return fmt.Errorf("hash mismatch! expected %s, got %s", chunk.Hash, calculatedHash)
	}

	// write chunk at precise disk offset concurrently
	return WriteChunkAt(outputPath, chunkData, chunk.Offset)
}
