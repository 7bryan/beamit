package engine

// 1 MB for LAN transfer, for avoiding HTTP header overhead
const DefaultChunkSize = 1024 * 1024 // size of each block (1 MB)

// ChunkMetadata, individual piece of file
type ChunkMetadata struct {
	ID     int    `json:"id"`     // chunk index
	Offset int64  `json:"offset"` // byte location of the chunk in the file
	Size   int64  `json:"size"`   // size in Bytes
	Hash   string `json:"hash"`   // SHA-256 fingerprint, verify data integrity
}

type FileManifest struct {
	FileName    string          `json:"file_name"`
	FileSize    int64           `json:"file_size"`
	ChunkSize   int64           `json:"chunk-size"`
	TotalChunks int             `json:"chunks"`
	Chunks      []ChunkMetadata `json:"chunks"`
}
