package engine

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

// TrackerResponse contains peer information from tracker
type TrackerResponse struct {
	Interval int        // Seconds between tracker announces
	Peers    []PeerAddr // List of peer addresses
}

// PeerAddr represents a peer's network address
type PeerAddr struct {
	IP   net.IP
	Port uint16
}

func (p PeerAddr) String() string {
	return fmt.Sprintf("%s:%d", p.IP, p.Port)
}

// TrackerClient manages communication with trackers
type TrackerClient struct {
	metaInfo *MetaInfo
	peerID   [20]byte
	port     uint16
}

// NewTrackerClient creates a tracker client
func NewTrackerClient(metaInfo *MetaInfo, port uint16) *TrackerClient {
	tc := &TrackerClient{
		metaInfo: metaInfo,
		port:     port,
	}

	// Generate random peer ID (20 bytes)
	// Format: -GO0001-<12 random bytes>
	copy(tc.peerID[:8], []byte("-GO0001-"))
	rand.Read(tc.peerID[8:])

	return tc
}

// Announce performs a tracker announce request
func (tc *TrackerClient) Announce(uploaded, downloaded, left int64, event string) (*TrackerResponse, error) {
	// Build tracker URL with query parameters
	params := url.Values{
		"info_hash":  {string(tc.metaInfo.InfoHash[:])},
		"peer_id":    {string(tc.peerID[:])},
		"port":       {fmt.Sprintf("%d", tc.port)},
		"uploaded":   {fmt.Sprintf("%d", uploaded)},
		"downloaded": {fmt.Sprintf("%d", downloaded)},
		"left":       {fmt.Sprintf("%d", left)},
		"compact":    {"1"}, // Request compact binary peer list
	}

	if event != "" {
		params.Set("event", event)
	}

	announceURL := tc.metaInfo.Announce + "?" + params.Encode()

	// Make HTTP GET request
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(announceURL)
	if err != nil {
		return nil, fmt.Errorf("tracker request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tracker returned status %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read tracker response: %w", err)
	}

	// Decode bencoded response
	decoder := NewBencodeDecoder(body)
	respVal, err := decoder.Decode()
	if err != nil {
		return nil, fmt.Errorf("failed to decode tracker response: %w", err)
	}

	respDict, ok := respVal.(BencodeDict)
	if !ok {
		return nil, fmt.Errorf("tracker response must be dictionary")
	}

	// Check for failure reason
	if failureReason, ok := respDict["failure reason"].(BencodeString); ok {
		return nil, fmt.Errorf("tracker error: %s", failureReason)
	}

	trackerResp := &TrackerResponse{}

	// Extract interval
	if interval, ok := respDict["interval"].(BencodeInt); ok {
		trackerResp.Interval = int(interval)
	}

	// Extract peers (compact binary format)
	if peersStr, ok := respDict["peers"].(BencodeString); ok {
		trackerResp.Peers = parseCompactPeers([]byte(peersStr))
	}

	return trackerResp, nil
}

// parseCompactPeers decodes binary peer format (6 bytes per peer: 4 IP + 2 port)
func parseCompactPeers(data []byte) []PeerAddr {
	if len(data)%6 != 0 {
		return nil
	}

	numPeers := len(data) / 6
	peers := make([]PeerAddr, numPeers)

	for i := 0; i < numPeers; i++ {
		offset := i * 6
		peers[i].IP = net.IPv4(data[offset], data[offset+1], data[offset+2], data[offset+3])
		peers[i].Port = binary.BigEndian.Uint16(data[offset+4 : offset+6])
	}

	return peers
}

// GetPeerID returns the client's peer ID
func (tc *TrackerClient) GetPeerID() [20]byte {
	return tc.peerID
}
