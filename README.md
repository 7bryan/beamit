# Beam It

## Project Structure

```
beamit/
├── cmd/
│   └── beamit/               # CLI Entry point
│       └── main.go           # Cobra root setup & CLI commands
├── pkg/
│   ├── discovery/            # Auto-discovery logic (mDNS / UDP broadcast)
│   │   ├── mdns.go           # Announces presence on LAN and listens for peers
│   │   └── peer.go           # Peer metadata definitions (ID, IP, Hostname, Port)
│   ├── engine/               # Transfer Engine & Chunking Logic
│   │   ├── chunker.go        # File partitioning, hashing (SHA256), and byte range management
│   │   ├── server.go         # HTTP Server for serving chunk requests
│   │   ├── client.go         # Concurrent worker pool making parallel chunk requests
│   │   └── state.go          # Resume state tracker (.aether download manifest files)
│   ├── transport/            # Network Abstraction Layer
│   │   ├── tcp.go            # High-speed local TCP connection handlers
│   │   └── webrtc.go         # WebRTC STUN/TURN fallback for non-LAN transfers
│   └── ui/                   # Terminal UI Layouts
│       ├── progress.go       # Bubbletea progress bars & transfer stats (MB/s, ETA)
│       └── picker.go         # File/Device picker component
├── mobile/                   # [Reserved for future mobile build]
│   └── android_ios/          # Flutter or Go Mobile / FFI bindings wrapper
├── docs/                     # Architecture diagrams & performance benchmarks (future plan)
├── .gitignore
├── go.mod
├── go.sum
└── README.md
```

### How the Client Worker Pool Works

```
                     [ Fetch /manifest ]
                              │
                    [ Preallocate File ]
                              │
                    [ Send Chunks to Channel ]
                              │
         ┌────────────────────┼────────────────────┐
         ▼                    ▼                    ▼
   [ Worker 1 ]         [ Worker 2 ]         [ Worker 3 ]
   GET bytes 0-1MB      GET bytes 1-2MB      GET bytes 2-3MB
         │                    │                    │
   Verify SHA-256       Verify SHA-256       Verify SHA-256
         │                    │                    │
   WriteAt(offset 0)    WriteAt(offset 1MB)  WriteAt(offset 2MB)
         └────────────────────┼────────────────────┘
                              ▼
                     [ Download Complete ]
```
