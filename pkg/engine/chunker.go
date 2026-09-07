package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// 1 MB for LAN transfer, for avoiding HTTP header overhead
const DefaultChunkSize = 1024 * 1024 // size of each block (1 MB)

// ChunkMetadata, individual piece of file
type ChunkMetadata struct {
	ID     int    `json:"id"`     // chunk index
	Offset int64  `json:"offset"` // byte location of the chunk in the file
	Size   int64  `json:"size"`   // size in Bytes
	Hash   string `json:"hash"`   // SHA-256 fingerprint, verify data integrity
}

// Blueprint of the file sent to the receiver before download
type FileManifest struct {
	FileName    string          `json:"file_name"`
	FileSize    int64           `json:"file_size"`
	ChunkSize   int64           `json:"chunk_size"`
	TotalChunks int             `json:"total_chunks"`
	Chunks      []ChunkMetadata `json:"chunks"`
}

// open a file and slices it into chunks,
// calculates SHA-256 hashes for each chunk,
// and builds a FileManifest
func GenerateManifest(filePath string) (*FileManifest, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close() // ensuring the file is closed after the function is finished

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	fileSize := fileInfo.Size()

	// calculate total chunks needed (rounding up for remaining bytes)
	totalChunks := int((fileSize + DefaultChunkSize - 1) / DefaultChunkSize)

	manifest := &FileManifest{
		FileName:    filepath.Base(filePath),
		FileSize:    fileSize,
		ChunkSize:   DefaultChunkSize,
		TotalChunks: totalChunks,
		Chunks:      make([]ChunkMetadata, 0, totalChunks),
	}

	// compute hashes of the file block block
	for i := 0; i < totalChunks; i++ {
		offset := int64(i) * DefaultChunkSize
		currentChunkSize := int64(DefaultChunkSize)

		// last chunk might be smaller than 1 MB
		if offset+currentChunkSize > fileSize {
			currentChunkSize = fileSize - offset
		}

		hash, err := calculateChunkHash(file, offset, currentChunkSize)
		if err != nil {
			return nil, fmt.Errorf("failed hashing chunk %d: %w", i, err)
		}

		manifest.Chunks = append(manifest.Chunks, ChunkMetadata{
			ID:     i,
			Offset: offset,
			Size:   currentChunkSize,
			Hash:   hash,
		})
	}

	return manifest, nil
}

// reads a specific section of a file and computes its SHA-256 hash
func calculateChunkHash(file *os.File, offset int64, size int64) (string, error) {
	// read a subset of a file cleanly without moving the main file pointer
	sectionReader := io.NewSectionReader(file, offset, size)

	hasher := sha256.New()

	// Stream bytes from section reader into the hasher
	if _, err := io.Copy(hasher, sectionReader); err != nil {
		return "", err
	}

	// convert raw binary hash to human-readable hexadecimal string
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// creates a blank file on the receiver disk filled with zeros
// mathinng the exact total size. Preventing fragmentation and ensures space exists
func PreallocateFile(outputPath string, size int64) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// truncate resizes the file immediately on disck to the desired lengths
	if err := file.Truncate(size); err != nil {
		return fmt.Errorf("failed to allocate disk space: %w", err)
	}

	return nil
}

// writes a chunk raw bytes directly into the preallocated file
// at its specifc bute offset
func WriteChunkAt(filePath string, data []byte, offset int64) error {
	// open file with write only mode
	file, err := os.OpenFile(filePath, os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file for writing: %w", err)
	}
	defer file.Close()

	// Write data at the exact byte offest regardless of current file position
	_, err = file.WriteAt(data, offset)
	if err != nil {
		return fmt.Errorf("failed writing chunk at offset %d: %w", offset, err)
	}

	return nil
}
