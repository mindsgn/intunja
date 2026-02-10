package mobile

import (
	"fmt"
	"sync"
	"time"

	"github.com/mindsgn-studio/intunja/engine"
)

// Client is the mobile-friendly BitTorrent client interface
// All methods use simple types compatible with gomobile
type Client struct {
	metaInfo *engine.MetaInfo
	manager  *engine.DownloadManager
	storage  *engine.StorageManager

	// Status
	mu            sync.RWMutex
	progress      float64
	downloadSpeed float64
	uploadSpeed   float64
	numPeers      int
	status        string // "starting", "downloading", "paused", "seeding", "stopped"

	// Statistics tracking
	lastDownloaded int64
	lastUploaded   int64
	lastUpdate     time.Time
}

// NewClient creates a new torrent client from a .torrent file
// torrentPath: path to .torrent file
// downloadDir: directory where files will be saved
func NewClient(torrentPath, downloadDir string) (*Client, error) {
	metaInfo, err := engine.ParseMetaInfo(torrentPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse torrent: %w", err)
	}

	manager := engine.NewDownloadManager(metaInfo, downloadDir)

	storage, err := engine.NewStorageManager(metaInfo, downloadDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage: %w", err)
	}

	c := &Client{
		metaInfo:   metaInfo,
		manager:    manager,
		storage:    storage,
		status:     "stopped",
		lastUpdate: time.Now(),
	}

	// Start statistics updater
	go c.updateStats()

	return c, nil
}

// Start begins downloading the torrent
func (c *Client) Start() error {
	c.mu.Lock()
	c.status = "starting"
	c.mu.Unlock()

	if err := c.manager.Start(); err != nil {
		c.mu.Lock()
		c.status = "error"
		c.mu.Unlock()
		return err
	}

	c.mu.Lock()
	c.status = "downloading"
	c.mu.Unlock()

	return nil
}

// Pause pauses the download
func (c *Client) Pause() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status = "paused"
	// Implementation would stop worker goroutines
}

// Resume resumes a paused download
func (c *Client) Resume() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.status == "paused" {
		c.status = "downloading"
		// Implementation would restart worker goroutines
	}
}

// Stop stops the download and closes all connections
func (c *Client) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.status = "stopped"

	// Flush any pending writes
	if err := c.storage.FlushBuffer(); err != nil {
		return err
	}

	// Close storage
	return c.storage.Close()
}

// GetProgress returns download progress as percentage (0-100)
func (c *Client) GetProgress() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.progress * 100.0
}

// GetDownloadSpeed returns current download speed in bytes per second
func (c *Client) GetDownloadSpeed() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.downloadSpeed
}

// GetUploadSpeed returns current upload speed in bytes per second
func (c *Client) GetUploadSpeed() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.uploadSpeed
}

// GetNumPeers returns number of connected peers
func (c *Client) GetNumPeers() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.numPeers
}

// GetStatus returns current status string
func (c *Client) GetStatus() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// GetTorrentName returns the name of the torrent
func (c *Client) GetTorrentName() string {
	return c.metaInfo.Info.Name
}

// GetTotalSize returns total size in bytes
func (c *Client) GetTotalSize() int64 {
	return c.metaInfo.TotalLength()
}

// GetDownloadedBytes returns total downloaded bytes
func (c *Client) GetDownloadedBytes() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastDownloaded
}

// GetUploadedBytes returns total uploaded bytes
func (c *Client) GetUploadedBytes() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastUploaded
}

// EnableSequentialMode enables sequential piece downloading for streaming
func (c *Client) EnableSequentialMode() {
	// Implementation would modify piece selection strategy
}

// SetMaxDownloadSpeed sets maximum download speed in bytes per second
// Use 0 for unlimited
func (c *Client) SetMaxDownloadSpeed(bytesPerSecond int64) {
	// Implementation would configure rate limiters
}

// SetMaxUploadSpeed sets maximum upload speed in bytes per second
// Use 0 for unlimited
func (c *Client) SetMaxUploadSpeed(bytesPerSecond int64) {
	// Implementation would configure rate limiters
}

// SetMaxPeers sets maximum number of peer connections
func (c *Client) SetMaxPeers(max int) {
	// Implementation would limit peer connections
}

// updateStats periodically updates statistics
func (c *Client) updateStats() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()

		if c.status != "downloading" && c.status != "seeding" {
			c.mu.Unlock()
			continue
		}

		// Update progress
		c.progress = c.manager.GetProgress()

		// Update statistics
		downloaded, uploaded, peers := c.manager.GetStats()

		// Calculate speeds
		now := time.Now()
		elapsed := now.Sub(c.lastUpdate).Seconds()

		if elapsed > 0 {
			c.downloadSpeed = float64(downloaded-c.lastDownloaded) / elapsed
			c.uploadSpeed = float64(uploaded-c.lastUploaded) / elapsed
		}

		c.lastDownloaded = downloaded
		c.lastUploaded = uploaded
		c.numPeers = peers
		c.lastUpdate = now

		// Check if complete
		if c.progress >= 1.0 && c.status == "downloading" {
			c.status = "seeding"
		}

		c.mu.Unlock()
	}
}

// FormatBytes formats bytes into human-readable string
// This is a helper function that can be called from mobile apps
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatSpeed formats speed into human-readable string
func FormatSpeed(bytesPerSecond float64) string {
	return FormatBytes(int64(bytesPerSecond)) + "/s"
}
