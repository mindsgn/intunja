package engine

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	BlockSize  = 16384 // 16KB per block (standard)
	MaxBacklog = 5     // Pipeline depth for requests
)

// PieceWork represents a piece to be downloaded
type PieceWork struct {
	Index  int
	Hash   [20]byte
	Length int
}

// PieceResult represents a completed piece
type PieceResult struct {
	Index int
	Data  []byte
	Error error
}

// DownloadManager orchestrates the download process
type DownloadManager struct {
	metaInfo      *MetaInfo
	trackerClient *TrackerClient
	peers         []*PeerConnection

	// Work queue
	workQueue chan *PieceWork
	results   chan *PieceResult

	// Download state
	downloaded []bool         // Which pieces we have
	pieceData  map[int][]byte // Cached piece data

	// Statistics
	totalDownloaded int64
	totalUploaded   int64

	// Configuration
	maxPeers     int
	downloadPath string

	mu   sync.Mutex
	done chan struct{}
}

// NewDownloadManager creates a download manager
func NewDownloadManager(metaInfo *MetaInfo, downloadPath string) *DownloadManager {
	numPieces := metaInfo.NumPieces()

	dm := &DownloadManager{
		metaInfo:     metaInfo,
		workQueue:    make(chan *PieceWork, numPieces),
		results:      make(chan *PieceResult),
		downloaded:   make([]bool, numPieces),
		pieceData:    make(map[int][]byte),
		maxPeers:     50,
		downloadPath: downloadPath,
		done:         make(chan struct{}),
	}

	// Initialize tracker client
	dm.trackerClient = NewTrackerClient(metaInfo, 6881)

	return dm
}

// Start begins the download process
func (dm *DownloadManager) Start() error {
	// Announce to tracker
	left := dm.metaInfo.TotalLength()
	trackerResp, err := dm.trackerClient.Announce(0, 0, left, "started")
	if err != nil {
		return fmt.Errorf("tracker announce failed: %w", err)
	}

	// Connect to peers
	peerID := dm.trackerClient.GetPeerID()
	for i, peerAddr := range trackerResp.Peers {
		if i >= dm.maxPeers {
			break
		}

		conn, err := NewPeerConnection(peerAddr, dm.metaInfo.InfoHash, peerID, 5*time.Second)
		if err != nil {
			continue // Skip failed connections
		}

		dm.peers = append(dm.peers, conn)
	}

	if len(dm.peers) == 0 {
		return errors.New("no peer connections established")
	}

	// Initialize work queue with all pieces
	go dm.queueWork()

	// Start peer workers
	for _, peer := range dm.peers {
		go dm.peerWorker(peer)
	}

	// Start result processor
	go dm.processResults()

	return nil
}

// queueWork populates the work queue with pieces to download
func (dm *DownloadManager) queueWork() {
	for i := 0; i < dm.metaInfo.NumPieces(); i++ {
		pieceLength := dm.calculatePieceLength(i)
		work := &PieceWork{
			Index:  i,
			Hash:   dm.metaInfo.Info.Pieces[i],
			Length: pieceLength,
		}
		dm.workQueue <- work
	}
}

// peerWorker downloads pieces from a single peer
func (dm *DownloadManager) peerWorker(peer *PeerConnection) {
	defer peer.Close()

	// Read initial bitfield
	msg, err := peer.ReadMessage()
	if err != nil {
		return
	}

	if msg != nil && msg.ID == MsgBitfield {
		peer.ParseBitfield(msg.Payload, dm.metaInfo.NumPieces())
	}

	// Send interested
	if err := peer.SendInterested(); err != nil {
		return
	}

	// Wait for unchoke
	for {
		msg, err := peer.ReadMessage()
		if err != nil {
			return
		}

		if msg == nil {
			continue // Keep-alive
		}

		if msg.ID == MsgUnchoke {
			peer.peerChoking = false
			break
		}
	}

	// Download loop
	for work := range dm.workQueue {
		// Check if peer has this piece
		if !peer.HasPiece(work.Index) {
			dm.workQueue <- work // Re-queue for another peer
			continue
		}

		// Download the piece
		data, err := dm.downloadPiece(peer, work)

		result := &PieceResult{
			Index: work.Index,
			Data:  data,
			Error: err,
		}

		dm.results <- result

		// If download failed, re-queue
		if err != nil {
			dm.workQueue <- work
			return // Disconnect from this peer
		}
	}
}

// downloadPiece downloads a single piece from a peer
func (dm *DownloadManager) downloadPiece(peer *PeerConnection, work *PieceWork) ([]byte, error) {
	pieceData := make([]byte, work.Length)
	downloaded := 0
	backlog := 0
	requested := 0

	for downloaded < work.Length {
		// Pipeline requests
		for backlog < MaxBacklog && requested < work.Length {
			blockSize := BlockSize
			if requested+blockSize > work.Length {
				blockSize = work.Length - requested
			}

			err := peer.RequestBlock(uint32(work.Index), uint32(requested), uint32(blockSize))
			if err != nil {
				return nil, err
			}

			backlog++
			requested += blockSize
		}

		// Wait for piece messages
		msg, err := peer.ReadMessage()
		if err != nil {
			return nil, err
		}

		if msg == nil {
			continue // Keep-alive
		}

		switch msg.ID {
		case MsgChoke:
			peer.peerChoking = true
			return nil, errors.New("peer choked us")

		case MsgPiece:
			// Parse piece message: <index><begin><block>
			if len(msg.Payload) < 8 {
				return nil, errors.New("piece message too short")
			}

			begin := int(uint32(msg.Payload[4])<<24 | uint32(msg.Payload[5])<<16 |
				uint32(msg.Payload[6])<<8 | uint32(msg.Payload[7]))
			block := msg.Payload[8:]

			copy(pieceData[begin:], block)
			downloaded += len(block)
			backlog--
		}
	}

	// Verify piece hash
	hash := sha1.Sum(pieceData)
	if hash != work.Hash {
		return nil, errors.New("piece hash verification failed")
	}

	return pieceData, nil
}

// processResults handles completed piece downloads
func (dm *DownloadManager) processResults() {
	numPieces := dm.metaInfo.NumPieces()
	completed := 0

	for result := range dm.results {
		if result.Error != nil {
			continue // Piece will be re-queued by worker
		}

		dm.mu.Lock()
		dm.downloaded[result.Index] = true
		dm.pieceData[result.Index] = result.Data
		dm.totalDownloaded += int64(len(result.Data))
		completed++
		dm.mu.Unlock()

		// Broadcast have message to all peers
		for _, peer := range dm.peers {
			peer.SendHave(uint32(result.Index))
		}

		// Check if download is complete
		if completed == numPieces {
			close(dm.done)
			return
		}
	}
}

// calculatePieceLength returns the length of a specific piece
func (dm *DownloadManager) calculatePieceLength(index int) int {
	totalLength := dm.metaInfo.TotalLength()
	pieceLength := dm.metaInfo.Info.PieceLength

	// Last piece may be shorter
	if int64(index+1)*pieceLength > totalLength {
		return int(totalLength - int64(index)*pieceLength)
	}

	return int(pieceLength)
}

// Wait blocks until download is complete
func (dm *DownloadManager) Wait() {
	<-dm.done
}

// GetProgress returns download progress (0.0 to 1.0)
func (dm *DownloadManager) GetProgress() float64 {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	completed := 0
	for _, done := range dm.downloaded {
		if done {
			completed++
		}
	}

	return float64(completed) / float64(len(dm.downloaded))
}

// GetStats returns download statistics
func (dm *DownloadManager) GetStats() (downloaded, uploaded int64, numPeers int) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	return dm.totalDownloaded, dm.totalUploaded, len(dm.peers)
}
