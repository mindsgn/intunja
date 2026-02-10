# Building a BitTorrent Client in Go: Complete Engineering Guide

**A Step-by-Step Tutorial for Junior Engineers**

---

## Table of Contents

1. [Introduction](#introduction)
2. [Prerequisites & Environment Setup](#prerequisites--environment-setup)
3. [Understanding the BitTorrent Protocol](#understanding-the-bittorrent-protocol)
4. [Phase 1: Bencode Serialization](#phase-1-bencode-serialization)
5. [Phase 2: Metainfo Parsing](#phase-2-metainfo-parsing)
6. [Phase 3: Tracker Communication](#phase-3-tracker-communication)
7. [Phase 4: Peer Wire Protocol](#phase-4-peer-wire-protocol)
8. [Phase 5: Download Manager & Work Queue](#phase-5-download-manager--work-queue)
9. [Phase 6: Storage Layer with Sparse Files](#phase-6-storage-layer-with-sparse-files)
10. [Phase 7: Building the TUI with Bubble Tea](#phase-7-building-the-tui-with-bubble-tea)
11. [Phase 8: Mobile Portability with gomobile](#phase-8-mobile-portability-with-gomobile)
12. [Advanced Topics](#advanced-topics)
13. [Troubleshooting Common Issues](#troubleshooting-common-issues)

---

## Introduction

### What You'll Build

You're building a production-grade BitTorrent client with:
- **Headless engine** that can run as a daemon or in mobile apps
- **Terminal User Interface (TUI)** using Bubble Tea for desktop use
- **Mobile support** via gomobile for Android and iOS
- **Advanced features**: streaming preview, file prioritization, integrated search

### Why This Matters

BitTorrent represents a fundamental shift in network architecture from centralized client-server models to peer-to-peer distribution. As more users join a swarm, the network becomes more capable—not less. This project teaches you:

- **Distributed systems**: Coordination without central authority
- **Concurrent programming**: Managing thousands of simultaneous connections
- **Network protocols**: Low-level TCP communication and binary serialization
- **State management**: Building responsive UIs that reflect asynchronous events

---

## Prerequisites & Environment Setup

### Required Software

1. **Go 1.21 or later**
   ```bash
   # Check your Go version
   go version
   
   # If you need to install Go:
   # Linux/Mac: Download from https://go.dev/dl/
   # Or use your package manager:
   sudo apt install golang-go  # Ubuntu/Debian
   brew install go             # macOS
   ```

2. **Git** (for version control)
   ```bash
   git --version
   ```

3. **A terminal emulator** with 256-color support

### Project Structure Setup

Create your project directory:

```bash
mkdir bittorrent-client
cd bittorrent-client

# Initialize Go module
go mod init github.com/yourusername/bittorrent-client

# Create directory structure
mkdir -p engine tui mobile docs
```

### Install Dependencies

```bash
# Bubble Tea for TUI
go get github.com/charmbracelet/bubbletea
go get github.com/charmbracelet/lipgloss
go get github.com/charmbracelet/bubbles

# Rate limiting
go get golang.org/x/time/rate
```

---

## Understanding the BitTorrent Protocol

### The Big Picture

BitTorrent uses a **swarm architecture**:

1. **Tracker**: A server that keeps a list of peers for each torrent
2. **Peers**: Clients downloading and uploading simultaneously
3. **Pieces**: Files are divided into chunks (typically 256KB or 512KB)
4. **Blocks**: Pieces are further divided into 16KB blocks for transfer

### Data Flow

```
.torrent file → Parser → Info Hash
     ↓
  Tracker → List of Peers
     ↓
  Connect to Peers → Handshake
     ↓
  Exchange Pieces → Verify with SHA-1
     ↓
  Write to Disk → Complete!
```

### Critical Concepts

**Info Hash**: A unique 20-byte SHA-1 hash that identifies a torrent. It's calculated from the bencoded "info" dictionary in the .torrent file. This hash is used everywhere—tracker announces, peer handshakes, DHT lookups.

**Bencoding**: A simple serialization format that's deterministic (same data always encodes the same way). This is critical for info-hash calculation.

**Choking/Unchoking**: BitTorrent's reciprocity mechanism. You unchoke peers that give you good download rates, implementing a "tit-for-tat" strategy.

---

## Phase 1: Bencode Serialization

### What is Bencode?

Bencode is BitTorrent's serialization format. It supports four types:

1. **Strings**: `4:spam` (length-prefixed)
2. **Integers**: `i42e` (i prefix, e suffix)
3. **Lists**: `l4:spami42ee` (l prefix, e suffix)
4. **Dictionaries**: `d3:key5:valuee` (d prefix, e suffix, alternating keys and values)

### Why Determinism Matters

The info-hash is calculated by taking the SHA-1 of the bencoded info dictionary. If you encode the same data differently (e.g., different key ordering), you get a different hash and can't join the swarm.

**Rule**: Dictionary keys MUST be sorted alphabetically.

### Implementation

See `engine/bencode.go` in the project. Key points:

```go
// BencodeDict ensures sorted keys
func (d BencodeDict) Encode() []byte {
    keys := make([]string, 0, len(d))
    for k := range d {
        keys = append(keys, k)
    }
    sort.Strings(keys)  // CRITICAL: alphabetical order
    
    // ... encode in sorted order
}
```

### Testing Your Bencode Implementation

Create a test file `engine/bencode_test.go`:

```go
package engine

import (
    "testing"
)

func TestBencodeString(t *testing.T) {
    s := BencodeString("spam")
    encoded := string(s.Encode())
    expected := "4:spam"
    
    if encoded != expected {
        t.Errorf("Expected %s, got %s", expected, encoded)
    }
}

func TestBencodeInt(t *testing.T) {
    i := BencodeInt(42)
    encoded := string(i.Encode())
    expected := "i42e"
    
    if encoded != expected {
        t.Errorf("Expected %s, got %s", expected, encoded)
    }
}

// Add more tests for lists and dictionaries
```

Run tests:
```bash
go test ./engine
```

---

## Phase 2: Metainfo Parsing

### Understanding .torrent Files

A .torrent file is a bencoded dictionary containing:

```
{
  "announce": "http://tracker.example.com:8080/announce",
  "info": {
    "name": "example.iso",
    "piece length": 262144,  // 256KB
    "pieces": "<concatenated 20-byte SHA-1 hashes>",
    "length": 1073741824     // For single-file torrents
  }
}
```

### The Info Dictionary

The "info" dictionary contains everything needed to verify the download:

- **name**: Suggested filename
- **piece length**: Size of each piece (power of 2)
- **pieces**: Concatenated SHA-1 hashes (20 bytes each)
- **length**: File size (single-file mode)
- **files**: Array of file objects (multi-file mode)

### Calculating the Info Hash

```go
// Extract info dictionary
infoVal := rootDict["info"]

// Encode it (preserving exact byte representation)
infoBytes := infoVal.Encode()

// Calculate SHA-1
hash := sha1.Sum(infoBytes)
```

**Critical**: You must hash the exact bytes from the .torrent file. Don't re-encode from a parsed structure—you might get different bytes due to encoding variations.

### Multi-File Torrents

Multi-file torrents have a "files" array instead of "length":

```
"info": {
  "name": "my_folder",
  "piece length": 262144,
  "pieces": "...",
  "files": [
    {
      "path": ["subfolder", "file1.txt"],
      "length": 1024
    },
    {
      "path": ["file2.txt"],
      "length": 2048
    }
  ]
}
```

See `engine/metainfo.go` for the complete implementation.

### Testing with a Real Torrent

Download a small public domain torrent file and test:

```bash
go run main.go test.torrent --daemon
```

---

## Phase 3: Tracker Communication

### The Tracker Protocol

Trackers maintain a list of peers for each torrent. The client announces to the tracker via HTTP GET:

```
http://tracker.example.com/announce?
  info_hash=<20-byte binary>&
  peer_id=<20-byte binary>&
  port=6881&
  uploaded=0&
  downloaded=0&
  left=1073741824&
  compact=1&
  event=started
```

### URL Encoding Binary Data

The info_hash and peer_id are 20-byte binary values. They must be URL-encoded:

```go
params := url.Values{
    "info_hash": {string(infoHash[:])},  // Go's url package handles encoding
    "peer_id":   {string(peerID[:])},
}
```

### Generating a Peer ID

Your peer ID identifies your client to others. Convention:

```
-XX####-<12 random bytes>
```

Where XX is a 2-letter code (e.g., "GO" for this client) and #### is version.

```go
copy(peerID[:8], []byte("-GO0001-"))
rand.Read(peerID[8:])
```

### Compact Peer Format

The tracker response includes a "peers" field. With `compact=1`, it's a binary string:

```
6 bytes per peer:
  - 4 bytes: IPv4 address
  - 2 bytes: Port (big-endian)
```

Example: `[192, 168, 1, 100, 0x1A, 0xE1]` → `192.168.1.100:6881`

```go
func parseCompactPeers(data []byte) []PeerAddr {
    numPeers := len(data) / 6
    peers := make([]PeerAddr, numPeers)
    
    for i := 0; i < numPeers; i++ {
        offset := i * 6
        ip := net.IPv4(data[offset], data[offset+1], 
                       data[offset+2], data[offset+3])
        port := binary.BigEndian.Uint16(data[offset+4:])
        peers[i] = PeerAddr{IP: ip, Port: port}
    }
    
    return peers
}
```

### Tracker Etiquette

The tracker response includes an "interval" field (in seconds). You MUST wait at least this long before the next announce. Typical values: 1800 seconds (30 minutes).

**Never** spam the tracker. Many trackers will ban clients that announce too frequently.

---

## Phase 4: Peer Wire Protocol

### The Handshake

Every peer connection starts with a 68-byte handshake:

```
Byte     Content
0        19 (protocol identifier length)
1-19     "BitTorrent protocol"
20-27    Reserved (8 zeros, used for extensions)
28-47    Info hash (20 bytes)
48-67    Peer ID (20 bytes)
```

### Handshake Validation

1. Check protocol identifier is "BitTorrent protocol"
2. Verify info hash matches (if not, disconnect immediately)
3. Extract remote peer ID

### Message Format

After handshake, all messages follow this format:

```
4 bytes: Length (big-endian)
1 byte:  Message ID
N bytes: Payload
```

**Keep-alive**: Length = 0 (no ID or payload)

### Core Messages

| ID | Name | Payload | Purpose |
|----|------|---------|---------|
| 0  | Choke | None | "I won't fulfill your requests" |
| 1  | Unchoke | None | "I'll fulfill your requests now" |
| 2  | Interested | None | "I want pieces from you" |
| 3  | Not Interested | None | "I don't need anything from you" |
| 4  | Have | 4-byte piece index | "I have this piece" |
| 5  | Bitfield | Bitfield of pieces | "Here's what I have" |
| 6  | Request | 12 bytes: index, begin, length | "Send me this block" |
| 7  | Piece | 8 bytes + block data | "Here's the block you requested" |
| 8  | Cancel | 12 bytes: index, begin, length | "Cancel my request" |

### Peer State Machine

Each connection has two boolean flags on each side:

1. **Am Choking**: Are we choking the peer? (Default: true)
2. **Am Interested**: Are we interested in the peer? (Default: false)
3. **Peer Choking**: Is the peer choking us? (Default: true)
4. **Peer Interested**: Is the peer interested in us? (Default: false)

**To download**: Send Interested, wait for Unchoke, then send Requests.

### Bitfield Message

After handshake, peers typically send a Bitfield message showing which pieces they have:

```
Payload: 1 bit per piece (packed into bytes)

Example for 20 pieces:
Bytes: [11111111 11111111 11110000]
       ^^^^^^^^ ^^^^^^^^ ^^^^----
       Pieces   Pieces   Pieces 16-19
       0-7      8-15     (rest are padding)
```

Parsing:

```go
for i := 0; i < numPieces; i++ {
    byteIndex := i / 8
    bitIndex := 7 - (i % 8)
    hasPiece := (bitfield[byteIndex] & (1 << bitIndex)) != 0
}
```

### Request Pipelining

**Critical for performance**: Don't wait for a response before sending the next request. Maintain a queue of 5 outstanding requests (the "backlog").

This keeps the peer's upload pipe saturated and overcomes network latency.

---

## Phase 5: Download Manager & Work Queue

### The Work Queue Pattern

Central to efficient downloading:

1. **Manager goroutine**: Maintains a queue of pieces to download
2. **Worker goroutines**: One per peer connection
3. **Results channel**: Workers send completed pieces back

```go
workQueue := make(chan *PieceWork, numPieces)
results := make(chan *PieceResult)

// Populate queue
for i := 0; i < numPieces; i++ {
    workQueue <- &PieceWork{Index: i, Hash: pieces[i]}
}

// Start workers
for _, peer := range peers {
    go peerWorker(peer, workQueue, results)
}
```

### Worker Logic

```go
func peerWorker(peer *PeerConnection, queue chan *PieceWork, results chan *PieceResult) {
    for work := range queue {
        // Check if peer has this piece
        if !peer.HasPiece(work.Index) {
            queue <- work  // Re-queue for another peer
            continue
        }
        
        // Download piece (blocks in loop)
        data, err := downloadPiece(peer, work)
        
        // Send result
        results <- &PieceResult{
            Index: work.Index,
            Data:  data,
            Error: err,
        }
        
        // If failed, re-queue
        if err != nil {
            queue <- work
            return  // Disconnect from peer
        }
    }
}
```

### Downloading a Piece

Pieces are too large for a single network packet (256KB typical). Break into 16KB blocks:

```go
func downloadPiece(peer *PeerConnection, work *PieceWork) ([]byte, error) {
    pieceData := make([]byte, work.Length)
    downloaded := 0
    requested := 0
    backlog := 0
    
    for downloaded < work.Length {
        // Pipeline requests
        for backlog < MaxBacklog && requested < work.Length {
            blockSize := min(BlockSize, work.Length - requested)
            peer.RequestBlock(work.Index, requested, blockSize)
            backlog++
            requested += blockSize
        }
        
        // Receive piece messages
        msg := peer.ReadMessage()
        if msg.ID == MsgPiece {
            // Extract block offset and data
            begin := parseBegin(msg.Payload)
            block := msg.Payload[8:]
            
            copy(pieceData[begin:], block)
            downloaded += len(block)
            backlog--
        }
    }
    
    // Verify hash
    if sha1.Sum(pieceData) != work.Hash {
        return nil, errors.New("hash mismatch")
    }
    
    return pieceData, nil
}
```

### Handling Failures

**Critical**: If a piece download fails (hash mismatch, peer disconnect), re-queue it for another worker.

**Never** trust data without verification. A malicious peer could send garbage.

---

## Phase 6: Storage Layer with Sparse Files

### Why Sparse Files?

Writing every 16KB block individually causes massive file fragmentation. Instead:

1. **Pre-allocate** the full file size
2. **Write zeros** only where needed (sparse file support)
3. **Buffer writes** in memory before flushing to disk

### Linux: ftruncate

```go
file, _ := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
file.Truncate(totalSize)  // Allocates space without writing
```

The OS allocates disk space but doesn't write zeros. A 10GB file is created in milliseconds.

### Windows: FSCTL_SET_SPARSE

On Windows NTFS, you must explicitly mark files as sparse:

```go
import "syscall"

// After creating file
handle := syscall.Handle(file.Fd())
var bytesReturned uint32
syscall.DeviceIoControl(
    handle,
    FSCTL_SET_SPARSE,  // Control code
    nil, 0,
    nil, 0,
    &bytesReturned,
    nil,
)
```

### Write Buffering

Don't write every piece immediately. Buffer in memory:

```go
type StorageManager struct {
    writeBuffer map[int][]byte  // pieceIndex -> data
    bufferMu    sync.Mutex
}

func (sm *StorageManager) WritePiece(index int, data []byte) error {
    sm.bufferMu.Lock()
    sm.writeBuffer[index] = data
    shouldFlush := len(sm.writeBuffer) >= 10
    sm.bufferMu.Unlock()
    
    if shouldFlush {
        return sm.FlushBuffer()
    }
    return nil
}
```

Flush every 10 pieces (~2.5MB for 256KB pieces). This batches writes and reduces syscall overhead.

### Multi-File Torrents

Pieces can span multiple files. Example:

```
File 1: 500KB
File 2: 800KB
Piece size: 256KB

Piece 0: Bytes 0-256KB     (entirely in File 1)
Piece 1: Bytes 256-512KB   (244KB in File 1, 12KB in File 2)
Piece 2: Bytes 512-768KB   (entirely in File 2)
```

You need to calculate which file(s) a piece belongs to:

```go
func (sm *StorageManager) writePieceToDisk(pieceIndex int, data []byte) error {
    offset := int64(pieceIndex) * pieceLength
    remaining := data
    
    for _, file := range sm.files {
        if offset < file.EndOffset {
            // This file contains part of the piece
            writeLen := min(len(remaining), file.EndOffset - offset)
            file.Handle.WriteAt(remaining[:writeLen], offset - file.StartOffset)
            
            remaining = remaining[writeLen:]
            offset += int64(writeLen)
            
            if len(remaining) == 0 {
                break
            }
        }
    }
    return nil
}
```

---

## Phase 7: Building the TUI with Bubble Tea

### The Elm Architecture (MVU)

Bubble Tea uses the Model-View-Update pattern:

1. **Model**: Application state (torrent list, selected index, etc.)
2. **View**: Renders the model as a string
3. **Update**: Processes messages and returns new model

### Basic Structure

```go
type Model struct {
    torrents    []*TorrentState
    selectedIdx int
}

func (m Model) Init() tea.Cmd {
    return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Handle keyboard input
    case tickMsg:
        // Update stats periodically
    }
    return m, nil
}

func (m Model) View() string {
    // Render UI as string
    return "BitTorrent Client\n\n" + m.renderTorrents()
}
```

### Handling Asynchronous Events

The download happens in background goroutines. How do you update the UI?

**Custom messages**:

```go
type pieceCompletedMsg struct {
    torrentID int
    pieceIndex int
}

// In background goroutine
func downloadWorker(torrents *Model) {
    // ... download piece ...
    
    // Send message to TUI
    p.Send(pieceCompletedMsg{
        torrentID: 0,
        pieceIndex: 42,
    })
}

// In Update function
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case pieceCompletedMsg:
        // Update model
        m.torrents[msg.torrentID].Progress++
    }
    return m, nil
}
```

### Periodic Updates

Use `tea.Tick` for regular updates:

```go
func tickCmd() tea.Cmd {
    return tea.Tick(time.Second, func(t time.Time) tea.Msg {
        return tickMsg(t)
    })
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg.(type) {
    case tickMsg:
        m.updateStats()
        return m, tickCmd()  // Schedule next tick
    }
    return m, nil
}
```

### Using Lipgloss for Styling

```go
import "github.com/charmbracelet/lipgloss"

var titleStyle = lipgloss.NewStyle().
    Bold(true).
    Foreground(lipgloss.Color("#7D56F4")).
    MarginBottom(1)

title := titleStyle.Render("BitTorrent Client")
```

### Tables with Bubbles

```go
import "github.com/charmbracelet/bubbles/table"

columns := []table.Column{
    {Title: "Name", Width: 40},
    {Title: "Progress", Width: 12},
    {Title: "Peers", Width: 8},
}

rows := []table.Row{
    {"example.iso", "45.2%", "12"},
}

t := table.New(
    table.WithColumns(columns),
    table.WithRows(rows),
    table.WithFocused(true),
)
```

---

## Phase 8: Mobile Portability with gomobile

### What is gomobile?

`gomobile` is a tool that generates Android (.aar) and iOS (.framework) packages from Go code. It allows your BitTorrent engine to run on mobile devices.

### Installing gomobile

```bash
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init
```

### Architecture for Mobile

Your engine must be **headless**—no TUI, no filesystem assumptions:

```
┌─────────────────────────────────┐
│   Mobile App (Java/Kotlin/Swift)│
├─────────────────────────────────┤
│   Engine Package (Go via gomobile)│
│   - Download Manager             │
│   - Peer connections             │
│   - Storage abstraction          │
└─────────────────────────────────┘
```

### Creating a Mobile-Friendly API

```go
package mobile

import "github.com/yourname/bittorrent-client/engine"

// TorrentClient is the mobile-facing API
type TorrentClient struct {
    manager *engine.DownloadManager
}

// NewClient creates a new torrent client
func NewClient(torrentPath string, downloadDir string) (*TorrentClient, error) {
    metaInfo, err := engine.ParseMetaInfo(torrentPath)
    if err != nil {
        return nil, err
    }
    
    manager := engine.NewDownloadManager(metaInfo, downloadDir)
    return &TorrentClient{manager: manager}, nil
}

// Start begins downloading
func (tc *TorrentClient) Start() error {
    return tc.manager.Start()
}

// GetProgress returns download progress (0-100)
func (tc *TorrentClient) GetProgress() float64 {
    return tc.manager.GetProgress() * 100
}
```

**Key rules for gomobile**:
- Export only simple types (int, float64, string, []byte)
- No channels, no complex structs
- Use callback functions for events

### Building for Android

```bash
# Create .aar package
gomobile bind -target=android -o bittorrent.aar ./mobile

# Output: bittorrent.aar and bittorrent-sources.jar
```

Use in Android Studio:
1. Copy `bittorrent.aar` to `app/libs/`
2. Add dependency in `build.gradle`:
   ```gradle
   dependencies {
       implementation(name: 'bittorrent', ext: 'aar')
   }
   ```

3. Use in Kotlin:
   ```kotlin
   import mobile.Mobile
   
   val client = Mobile.newClient("/path/to/torrent", "/Download")
   client.start()
   
   // Check progress
   val progress = client.getProgress()
   ```

### Building for iOS

```bash
# Create .framework package
gomobile bind -target=ios -o BitTorrent.framework ./mobile
```

Use in Xcode:
1. Drag `BitTorrent.framework` into project
2. Import in Swift:
   ```swift
   import BitTorrent
   
   let client = MobileNewClient("/path/to/torrent", "/Download")
   try! client?.start()
   ```

### Background Execution on Mobile

**Android**: Use WorkManager for background downloads:

```kotlin
class DownloadWorker(context: Context, params: WorkerParameters) 
    : Worker(context, params) {
    
    override fun doWork(): Result {
        val client = Mobile.newClient(torrentPath, downloadDir)
        client.start()
        
        while (client.getProgress() < 100) {
            Thread.sleep(1000)
        }
        
        return Result.success()
    }
}
```

**iOS**: Use BGTaskScheduler:

```swift
BGTaskScheduler.shared.register(
    forTaskWithIdentifier: "com.app.download",
    using: nil
) { task in
    let client = MobileNewClient(torrentPath, downloadDir)
    try! client?.start()
}
```

---

## Advanced Topics

### Sequential Piece Selection for Streaming

For video/audio preview, override the rarest-first strategy:

```go
func (dm *DownloadManager) EnableSequentialMode() {
    // Prioritize pieces in order
    for i := 0; i < dm.metaInfo.NumPieces(); i++ {
        dm.workQueue <- &PieceWork{Index: i, ...}
    }
}
```

**Header/footer priority**: MP4 and MKV files store metadata at the beginning and end. Download these first:

```go
// Download first 5 pieces
for i := 0; i < 5; i++ {
    dm.workQueue <- &PieceWork{Index: i, Priority: High}
}

// Download last 5 pieces
numPieces := dm.metaInfo.NumPieces()
for i := numPieces - 5; i < numPieces; i++ {
    dm.workQueue <- &PieceWork{Index: i, Priority: High}
}

// Then fill in the middle sequentially
```

### File Selection & Prioritization

For multi-file torrents, users may want to skip files:

```go
// Calculate which pieces belong to which files
func (sm *StorageManager) GetFilePieces(fileIndex int) []int {
    file := sm.metaInfo.Info.Files[fileIndex]
    
    startByte := file.StartOffset
    endByte := startByte + file.Length
    
    pieceLength := sm.metaInfo.Info.PieceLength
    
    startPiece := int(startByte / pieceLength)
    endPiece := int((endByte - 1) / pieceLength)
    
    pieces := make([]int, 0)
    for i := startPiece; i <= endPiece; i++ {
        pieces = append(pieces, i)
    }
    
    return pieces
}

// Skip a file by removing its pieces from the queue
func (dm *DownloadManager) SkipFile(fileIndex int) {
    piecesToSkip := dm.storage.GetFilePieces(fileIndex)
    
    // Remove from work queue (implementation depends on your queue structure)
    dm.skipPieces(piecesToSkip)
}
```

### Bandwidth Throttling with Token Bucket

Use `golang.org/x/time/rate` to limit upload/download speed:

```go
import "golang.org/x/time/rate"

type ThrottledConn struct {
    net.Conn
    readLimiter  *rate.Limiter
    writeLimiter *rate.Limiter
}

func (tc *ThrottledConn) Read(p []byte) (n int, err error) {
    // Wait for token
    tc.readLimiter.WaitN(context.Background(), len(p))
    return tc.Conn.Read(p)
}

func (tc *ThrottledConn) Write(p []byte) (n int, err error) {
    tc.writeLimiter.WaitN(context.Background(), len(p))
    return tc.Conn.Write(p)
}

// Create with speed limits (bytes per second)
maxDownload := 1024 * 1024  // 1 MB/s
maxUpload := 512 * 1024     // 512 KB/s

conn := &ThrottledConn{
    Conn:         rawConn,
    readLimiter:  rate.NewLimiter(rate.Limit(maxDownload), maxDownload),
    writeLimiter: rate.NewLimiter(rate.Limit(maxUpload), maxUpload),
}
```

### Integrated Search with Jackett

Jackett aggregates torrent trackers into a unified API (Torznab):

```go
func searchTorrents(query string) ([]TorrentResult, error) {
    // Jackett API endpoint
    url := fmt.Sprintf("http://localhost:9117/api/v2.0/indexers/all/results?apikey=%s&t=search&q=%s",
        jackettAPIKey,
        url.QueryEscape(query))
    
    resp, err := http.Get(url)
    // ... parse XML response
}
```

### Backpressure Handling

**Problem**: Network is faster than disk writes. Memory fills with pending pieces.

**Solution**: Limit in-flight pieces:

```go
type DownloadManager struct {
    inFlightSemaphore chan struct{}  // Limit concurrent downloads
}

func (dm *DownloadManager) downloadPiece(work *PieceWork) {
    // Acquire semaphore
    dm.inFlightSemaphore <- struct{}{}
    defer func() { <-dm.inFlightSemaphore }()
    
    // Download piece...
}

// Initialize with limit
dm.inFlightSemaphore = make(chan struct{}, 20)  // Max 20 pieces in memory
```

### Snubbed Peer Detection

A peer is "snubbed" if they don't send data for 60 seconds despite being unchoked:

```go
type PeerConnection struct {
    lastReceived time.Time
}

func (pc *PeerConnection) IsSnubbed() bool {
    return time.Since(pc.lastReceived) > 60*time.Second && !pc.peerChoking
}

// In download manager, deprioritize snubbed peers
```

---

## Troubleshooting Common Issues

### Issue: "Info hash mismatch" errors

**Cause**: Your bencode encoder is not deterministic. Dictionary keys aren't sorted.

**Fix**: Ensure `BencodeDict.Encode()` sorts keys alphabetically.

### Issue: "Piece hash verification failed"

**Causes**:
1. Network corruption (rare with TCP)
2. Malicious peer sending bad data
3. Bug in piece assembly (wrong offset)

**Debug**:
```go
// Log the piece data
fmt.Printf("Expected: %x\n", expectedHash)
fmt.Printf("Got:      %x\n", sha1.Sum(pieceData))

// Check offsets
for each block {
    fmt.Printf("Block at offset %d, length %d\n", begin, len(block))
}
```

### Issue: Can't connect to any peers

**Causes**:
1. Firewall blocking port 6881
2. NAT not forwarded
3. Tracker returned no peers

**Fix**:
```bash
# Check if port is open
nc -zv localhost 6881

# Try a different port
./bittorrent-client --port 8080

# Enable DHT for trackerless discovery
```

### Issue: Download stalls at X%

**Causes**:
1. All connected peers are missing the same piece (rare piece problem)
2. Peers disconnecting
3. Snubbed by peers

**Fix**:
- Implement "endgame mode": Request missing pieces from ALL peers
- Connect to more peers from tracker
- Use DHT to find additional peers

### Issue: High memory usage

**Cause**: Too many pieces buffered before writing to disk.

**Fix**: Reduce buffer size in StorageManager:

```go
shouldFlush := len(sm.writeBuffer) >= 5  // Flush more frequently
```

### Issue: TUI not updating

**Cause**: Background goroutines aren't sending messages to Bubble Tea program.

**Fix**: Ensure you have a reference to the program:

```go
var tuiProgram *tea.Program

func startDownload() {
    go func() {
        // ... download piece ...
        tuiProgram.Send(pieceCompletedMsg{...})
    }()
}

func main() {
    tuiProgram = tea.NewProgram(model)
    tuiProgram.Run()
}
```

### Issue: gomobile build fails with "unsupported type"

**Cause**: You're exporting a type that gomobile doesn't support (channels, complex structs).

**Fix**: Create a wrapper with only simple types:

```go
// Don't export this
type ComplexResult struct {
    Channel chan int
    Map     map[string]interface{}
}

// Export this instead
type SimpleResult struct {
    Value   int
    Message string
}
```

---

## Summary & Next Steps

You've built a complete BitTorrent client with:
- ✅ Bencode serialization
- ✅ Metainfo parsing
- ✅ Tracker communication
- ✅ Peer wire protocol
- ✅ Concurrent downloads with work queues
- ✅ Sparse file storage
- ✅ Terminal UI with Bubble Tea
- ✅ Mobile compatibility via gomobile

### Further Enhancements

1. **DHT support** (BEP 5): Trackerless peer discovery
2. **PEX** (BEP 11): Peer exchange for rapid swarm expansion
3. **Magnet links** (BEP 9): Download without .torrent file
4. **uTP protocol**: Congestion-controlled transport
5. **Encryption**: Avoid ISP throttling
6. **Web UI**: Control via browser
7. **RSS/Auto-download**: Monitor feeds for new releases

### Testing with Public Torrents

Test your client with legal torrents:
- Linux ISOs: https://ubuntu.com/download/alternative-downloads
- Public domain content: https://archive.org
- Creative Commons: https://vodo.net

**Never** use your client for piracy. Respect intellectual property.

### Contributing & Community

- Read the BEP specifications: https://www.bittorrent.org/beps/bep_0000.html
- Join BitTorrent developer communities
- Open source your work (consider MIT or Apache 2.0 license)

---

**Good luck with your BitTorrent client! You're now equipped to build production-grade P2P systems.**