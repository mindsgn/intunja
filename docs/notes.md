# BitTorrent Client: Senior Engineering Notes

This document addresses advanced technical questions and design decisions for senior engineers reviewing or contributing to this project.

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Backpressure Management](#backpressure-management)
3. [Streaming Strategy](#streaming-strategy)
4. [Peer State Management](#peer-state-management)
5. [Performance Optimization](#performance-optimization)
6. [Mobile Portability Challenges](#mobile-portability-challenges)
7. [Security Considerations](#security-considerations)
8. [Failure Modes & Recovery](#failure-modes--recovery)

---

## Architecture Overview

### Design Philosophy: Separation of Concerns

The client follows a strict layered architecture:

```
┌─────────────────────────────────────────┐
│  Presentation Layer (TUI/Mobile UI)     │
├─────────────────────────────────────────┤
│  Application Layer (Download Manager)   │
├─────────────────────────────────────────┤
│  Protocol Layer (Peer Wire, Tracker)    │
├─────────────────────────────────────────┤
│  Storage Layer (Sparse Files, Caching)  │
└─────────────────────────────────────────┘
```

**Key principle**: The engine is completely headless. The TUI communicates via channels/callbacks, allowing the same engine to run:
- As a daemon on servers
- Inside mobile apps (via gomobile)
- With alternative UIs (web, GUI)

### Concurrency Model

We use the **actor pattern** with Go's goroutines and channels:

1. **Manager Actor**: Owns the work queue, distributes pieces
2. **Worker Actors**: One per peer, downloads pieces independently
3. **Storage Actor**: Handles all disk I/O operations

Communication via typed channels ensures no shared mutable state, preventing race conditions.

---

## Backpressure Management

### The Problem

Network bandwidth often exceeds disk write speed, especially on:
- HDDs (sequential write: ~100-150 MB/s)
- Mobile flash storage (varies widely)
- Network storage (NFS, SMB)

If we don't apply backpressure, memory usage becomes unbounded as pieces queue for disk write.

### Solution 1: Semaphore-Based Limiting

```go
type DownloadManager struct {
    inFlightSemaphore chan struct{} // Buffered channel as semaphore
}

func NewDownloadManager(...) *DownloadManager {
    return &DownloadManager{
        inFlightSemaphore: make(chan struct{}, 20), // Max 20 pieces in flight
    }
}

func (dm *DownloadManager) downloadPiece(work *PieceWork) ([]byte, error) {
    // Acquire semaphore (blocks if 20 pieces already in flight)
    dm.inFlightSemaphore <- struct{}{}
    defer func() { <-dm.inFlightSemaphore }()
    
    // Download piece...
}
```

**Why 20 pieces?**
- With 256KB pieces: 20 × 256KB = 5MB in memory
- With 512KB pieces: 20 × 512KB = 10MB in memory
- Balances memory usage with download efficiency

### Solution 2: Write Buffer Flushing

The storage manager batches writes:

```go
type StorageManager struct {
    writeBuffer map[int][]byte
    flushSize   int  // Flush after N pieces
}

func (sm *StorageManager) WritePiece(index int, data []byte) error {
    sm.bufferMu.Lock()
    sm.writeBuffer[index] = data
    shouldFlush := len(sm.writeBuffer) >= sm.flushSize
    sm.bufferMu.Unlock()
    
    if shouldFlush {
        return sm.flushToDisk()
    }
    return nil
}
```

**Tuning the flush size:**
- Too small (1-2 pieces): Frequent small writes, poor performance
- Too large (50+ pieces): High memory usage
- Sweet spot: 10 pieces (~2.5-5MB batch)

### Solution 3: Dynamic Adjustment

Advanced implementation monitors disk write latency:

```go
func (sm *StorageManager) adjustFlushSize() {
    writeLatency := sm.measureWriteLatency()
    
    if writeLatency > 100*time.Millisecond {
        // Disk is slow, reduce buffer to apply backpressure faster
        sm.flushSize = max(5, sm.flushSize/2)
    } else if writeLatency < 10*time.Millisecond {
        // Disk is fast, increase buffer for efficiency
        sm.flushSize = min(50, sm.flushSize*2)
    }
}
```

### Monitoring Backpressure

Expose metrics to detect backpressure:

```go
type DownloadStats struct {
    InFlightPieces    int     // Current pieces being downloaded
    QueuedPieces      int     // Pieces waiting in buffer
    AvgWriteLatency   float64 // Average disk write time
}

func (dm *DownloadManager) GetBackpressureMetrics() BackpressureMetrics {
    return BackpressureMetrics{
        InFlight: len(dm.inFlightSemaphore),
        Queued:   len(dm.storage.writeBuffer),
        // If InFlight is consistently at max, we're backpressure-limited
        IsBackpressured: len(dm.inFlightSemaphore) >= cap(dm.inFlightSemaphore),
    }
}
```

---

## Streaming Strategy

### The MP4/MKV Metadata Problem

Video containers store critical metadata (the "moov atom" in MP4, "SeekHead" in MKV) that players need to begin playback:

```
MP4 File Structure:
┌──────────────┐
│ ftyp (header)│ ← Essential
├──────────────┤
│ moov (index) │ ← CRITICAL for playback
├──────────────┤
│ mdat (video) │ ← Actual data
│  ...         │
│  ...         │
└──────────────┘

OR (web-optimized):
┌──────────────┐
│ ftyp         │
├──────────────┤
│ mdat         │
│  ...         │
├──────────────┤
│ moov         │ ← At end in non-optimized files
└──────────────┘
```

### Piece Prioritization Algorithm

```go
type PiecePriority int

const (
    PriorityLow    PiecePriority = 0
    PriorityNormal PiecePriority = 1
    PriorityHigh   PiecePriority = 2
)

type PieceWork struct {
    Index    int
    Hash     [20]byte
    Length   int
    Priority PiecePriority
}

func (dm *DownloadManager) EnableStreamingMode() {
    numPieces := dm.metaInfo.NumPieces()
    
    // Phase 1: Download first 5 pieces (header + moov if at start)
    for i := 0; i < min(5, numPieces); i++ {
        dm.queuePieceWithPriority(i, PriorityHigh)
    }
    
    // Phase 2: Download last 5 pieces (moov if at end)
    for i := max(0, numPieces-5); i < numPieces; i++ {
        dm.queuePieceWithPriority(i, PriorityHigh)
    }
    
    // Phase 3: Sequential for middle (enables progressive playback)
    for i := 5; i < numPieces-5; i++ {
        dm.queuePieceWithPriority(i, PriorityNormal)
    }
}
```

### Piece Selection in Worker

Workers respect priority:

```go
func (dm *DownloadManager) selectNextPiece() *PieceWork {
    // Priority queue implementation
    var highPriority, normalPriority, lowPriority []*PieceWork
    
    // Scan work queue
    select {
    case work := <-dm.highPriorityQueue:
        return work
    default:
        select {
        case work := <-dm.normalPriorityQueue:
            return work
        default:
            return <-dm.lowPriorityQueue
        }
    }
}
```

### Header Detection

To automatically detect if a file needs streaming:

```go
func (dm *DownloadManager) isStreamable(metaInfo *MetaInfo) bool {
    name := strings.ToLower(metaInfo.Info.Name)
    
    streamableExtensions := []string{".mp4", ".mkv", ".avi", ".mov", ".webm"}
    
    for _, ext := range streamableExtensions {
        if strings.HasSuffix(name, ext) {
            return true
        }
    }
    
    return false
}
```

### Progressive Verification

For streaming to work, we need partial file verification:

```go
func (dm *DownloadManager) canStartPlayback() bool {
    // Check if first 5 pieces are complete
    for i := 0; i < 5; i++ {
        if !dm.downloaded[i] {
            return false
        }
    }
    
    // Check if last 5 pieces are complete (for moov)
    numPieces := dm.metaInfo.NumPieces()
    for i := numPieces - 5; i < numPieces; i++ {
        if !dm.downloaded[i] {
            return false
        }
    }
    
    return true
}
```

---

## Peer State Management

### The Snubbed Peer Problem

A peer is "snubbed" when they:
1. Are not choking us (`peerChoking == false`)
2. Have not sent data in 60 seconds

This indicates the peer accepted our interest but isn't uploading, possibly because:
- They're uploading to other peers (we're not in their top 4)
- Network congestion
- Intentionally malicious behavior

### Detection

```go
type PeerConnection struct {
    lastDataReceived time.Time
    peerChoking      bool
    
    // Statistics
    downloadRate     float64 // Bytes per second
    uploadRate       float64
}

func (pc *PeerConnection) IsSnubbed() bool {
    if pc.peerChoking {
        return false // Not snubbed if they're explicitly choking us
    }
    
    timeSinceData := time.Since(pc.lastDataReceived)
    return timeSinceData > 60*time.Second
}

func (pc *PeerConnection) UpdateDataReceived(bytes int) {
    pc.lastDataReceived = time.Now()
    
    // Update download rate (exponential moving average)
    elapsed := time.Since(pc.lastUpdate).Seconds()
    instantRate := float64(bytes) / elapsed
    
    alpha := 0.2 // Smoothing factor
    pc.downloadRate = alpha*instantRate + (1-alpha)*pc.downloadRate
}
```

### Response Strategy

```go
func (dm *DownloadManager) handleSnubbedPeers() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        for _, peer := range dm.peers {
            if peer.IsSnubbed() {
                dm.handleSnubbedPeer(peer)
            }
        }
    }
}

func (dm *DownloadManager) handleSnubbedPeer(peer *PeerConnection) {
    // Don't immediately disconnect - peer might recover
    
    // Strategy 1: Reduce priority for piece assignment
    peer.priority = PriorityLow
    
    // Strategy 2: Send "not interested" to free up their upload slot
    peer.SendNotInterested()
    
    // Strategy 3: After 3 minutes of being snubbed, disconnect
    if time.Since(peer.lastDataReceived) > 3*time.Minute {
        peer.Close()
        dm.removePeer(peer)
    }
}
```

### Optimistic Unchoking

To avoid getting stuck with slow peers, we periodically try new ones:

```go
func (dm *DownloadManager) optimisticUnchoke() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        // Find a random choked peer
        chokedPeers := dm.getChokedPeers()
        if len(chokedPeers) == 0 {
            continue
        }
        
        randomPeer := chokedPeers[rand.Intn(len(chokedPeers))]
        
        // Unchoke them regardless of rate
        randomPeer.SendUnchoke()
        randomPeer.SendInterested()
        
        // Give them 30 seconds to prove themselves
        // If they're fast, they'll stay unchoked in the next iteration
    }
}
```

### Choke Algorithm (Tit-for-Tat)

```go
func (dm *DownloadManager) chokeAlgorithm() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for range ticker.C {
        // Sort peers by download rate
        sort.Slice(dm.peers, func(i, j int) bool {
            return dm.peers[i].downloadRate > dm.peers[j].downloadRate
        })
        
        // Unchoke top 4 uploaders
        for i := 0; i < min(4, len(dm.peers)); i++ {
            if dm.peers[i].amChoking {
                dm.peers[i].SendUnchoke()
                dm.peers[i].amChoking = false
            }
        }
        
        // Choke everyone else
        for i := 4; i < len(dm.peers); i++ {
            if !dm.peers[i].amChoking {
                dm.peers[i].SendChoke()
                dm.peers[i].amChoking = true
            }
        }
    }
}
```

---

## Performance Optimization

### Memory Profiling

Use Go's pprof to identify memory bottlenecks:

```go
import _ "net/http/pprof"

func main() {
    go func() {
        http.ListenAndServe("localhost:6060", nil)
    }()
    
    // Rest of application...
}
```

Access profiles at:
- `http://localhost:6060/debug/pprof/heap` - Memory allocations
- `http://localhost:6060/debug/pprof/goroutine` - Goroutine count

### CPU Profiling

```bash
go test -cpuprofile=cpu.prof -bench=.
go tool pprof cpu.prof
```

### Benchmarking Bencode

```go
func BenchmarkBencodeEncode(b *testing.B) {
    dict := BencodeDict{
        "announce": BencodeString("http://tracker.example.com"),
        "info": BencodeDict{
            "name":         BencodeString("example.iso"),
            "piece length": BencodeInt(262144),
        },
    }
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = dict.Encode()
    }
}
```

Expected results:
- ~1-2 μs per encode for typical .torrent metadata
- ~10-20 μs for large dictionaries (1000+ keys)

### Zero-Copy I/O

For serving pieces to peers, avoid copying data:

```go
func (sm *StorageManager) ServePieceToPeer(pieceIndex int, conn net.Conn) error {
    // Use io.Copy with sendfile syscall on Linux
    piece, _ := sm.ReadPiece(pieceIndex)
    
    // This uses sendfile internally for zero-copy
    _, err := conn.Write(piece)
    return err
}
```

---

## Mobile Portability Challenges

### Platform-Specific Limitations

**Android:**
- Background execution restricted (use WorkManager)
- Storage access permissions required
- No sparse file support on all filesystems (FAT32 on SD cards)

**iOS:**
- Even more restrictive background execution
- Must use BGTaskScheduler
- Limited to 30 seconds of background time unless audio/location

### Cross-Platform Abstractions

```go
// Platform-agnostic storage interface
type Storage interface {
    AllocateFile(path string, size int64) error
    WriteAt(data []byte, offset int64) error
    ReadAt(data []byte, offset int64) error
}

// Linux implementation
type LinuxStorage struct {
    file *os.File
}

func (ls *LinuxStorage) AllocateFile(path string, size int64) error {
    // Use ftruncate for sparse files
    return ls.file.Truncate(size)
}

// Android implementation
type AndroidStorage struct {
    file *os.File
}

func (as *AndroidStorage) AllocateFile(path string, size int64) error {
    // Check if sparse files are supported
    if isSparseFSSupported() {
        return as.file.Truncate(size)
    }
    
    // Fallback: pre-allocate with zeros (slower)
    return writeZeros(as.file, size)
}
```

### gomobile Limitations

What you **cannot** export:
- Channels
- Functions (except as callbacks)
- Interfaces (except for specific cases)
- Complex nested structs

What you **can** export:
- Primitives (int, float64, bool, string)
- Byte slices ([]byte)
- Simple structs with exported fields
- Methods on exported structs

**Good:**
```go
type Client struct {
    name string
}

func (c *Client) GetName() string {
    return c.name
}
```

**Bad:**
```go
type Client struct {
    results chan *Result  // ❌ Channel
}

func (c *Client) OnComplete(cb func(Result)) {  // ❌ Function parameter
    // ...
}
```

**Workaround for callbacks:**
```go
// Define interface in mobile package
type ProgressListener interface {
    OnProgress(percent float64)
}

// Client accepts listener
type Client struct {
    listener ProgressListener
}

func (c *Client) SetProgressListener(listener ProgressListener) {
    c.listener = listener
}

// Then in Android/iOS, implement the interface
```

---

## Security Considerations

### Info Hash Validation

Always verify the info hash in peer handshakes:

```go
func (pc *PeerConnection) handshake() error {
    // ... receive handshake ...
    
    var receivedHash [20]byte
    copy(receivedHash[:], response[28:48])
    
    if receivedHash != pc.infoHash {
        return errors.New("info hash mismatch - potential attack")
    }
    
    return nil
}
```

**Why this matters:** A malicious peer could try to send data for a different torrent, potentially:
- Corrupting your download
- Filling your disk with unwanted data
- Tricking you into downloading malware

### Piece Verification

**Never** skip SHA-1 verification:

```go
func (dm *DownloadManager) verifyPiece(index int, data []byte) error {
    hash := sha1.Sum(data)
    
    if hash != dm.metaInfo.Info.Pieces[index] {
        // Log the peer for potential banning
        dm.logBadPeer(currentPeer, "sent invalid piece")
        
        return errors.New("piece verification failed")
    }
    
    return nil
}
```

### DHT Security (BEP 42)

When implementing DHT, use BEP 42 for node ID restrictions to prevent Sybil attacks:

```go
func generateSecureNodeID(ip net.IP) [20]byte {
    // Node ID must match IP address prefix (BEP 42)
    var id [20]byte
    
    // Use IP prefix for first bytes
    rand := sha1.Sum(append(ip, randomBytes()...))
    copy(id[:], rand[:])
    
    return id
}
```

---

## Failure Modes & Recovery

### Scenario 1: Tracker Offline

**Detection:**
```go
_, err := dm.trackerClient.Announce(...)
if err != nil {
    // Tracker unreachable
}
```

**Recovery:**
1. Try announce-list (BEP 12) - alternate trackers
2. Fall back to DHT (if enabled)
3. Use PEX from existing peers

### Scenario 2: Corrupt Piece

**Detection:**
```go
if sha1.Sum(piece) != expectedHash {
    // Piece is corrupt
}
```

**Recovery:**
1. Re-queue the piece
2. Request from different peer
3. Log the peer (potential ban after 3 bad pieces)

### Scenario 3: Disk Full

**Detection:**
```go
_, err := file.Write(data)
if err == syscall.ENOSPC {
    // Disk full
}
```

**Recovery:**
1. Pause download immediately
2. Notify user
3. Flush buffers to ensure consistency
4. Optionally: delete incomplete pieces to free space

### Scenario 4: Network Partition

All peers disconnect simultaneously.

**Detection:**
```go
func (dm *DownloadManager) monitorPeerHealth() {
    if len(dm.getConnectedPeers()) == 0 {
        // All peers disconnected
    }
}
```

**Recovery:**
1. Re-announce to tracker
2. Initiate new DHT queries
3. Check if local network is down

---

## Conclusion

This BitTorrent client demonstrates production-grade distributed systems engineering:

- **Concurrency**: Actor model with goroutines
- **Backpressure**: Semaphore-based flow control
- **Performance**: Zero-copy I/O, sparse files, write buffering
- **Portability**: Clean abstractions for mobile
- **Security**: Cryptographic verification at every layer
- **Resilience**: Graceful degradation and recovery

**For senior engineers**: This codebase serves as a reference for building scalable P2P systems. The patterns used here (work queues, backpressure, piece prioritization) apply to any distributed data transfer system.