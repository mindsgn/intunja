package engine

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// Message types (peer wire protocol)
const (
	MsgChoke         = 0
	MsgUnchoke       = 1
	MsgInterested    = 2
	MsgNotInterested = 3
	MsgHave          = 4
	MsgBitfield      = 5
	MsgRequest       = 6
	MsgPiece         = 7
	MsgCancel        = 8
)

// PeerMessage represents a peer wire protocol message
type PeerMessage struct {
	ID      byte
	Payload []byte
}

// PeerConnection manages a connection to a single peer
type PeerConnection struct {
	conn         net.Conn
	addr         PeerAddr
	infoHash     [20]byte
	peerID       [20]byte
	remotePeerID [20]byte

	// State
	amChoking      bool // Are we choking the peer?
	amInterested   bool // Are we interested in the peer?
	peerChoking    bool // Is the peer choking us?
	peerInterested bool // Is the peer interested in us?

	bitfield []bool // Which pieces the peer has
}

// NewPeerConnection establishes a connection to a peer
func NewPeerConnection(addr PeerAddr, infoHash, peerID [20]byte, timeout time.Duration) (*PeerConnection, error) {
	conn, err := net.DialTimeout("tcp", addr.String(), timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	pc := &PeerConnection{
		conn:        conn,
		addr:        addr,
		infoHash:    infoHash,
		peerID:      peerID,
		amChoking:   true,
		peerChoking: true,
	}

	// Perform handshake
	if err := pc.handshake(); err != nil {
		conn.Close()
		return nil, err
	}

	return pc, nil
}

// handshake performs the BitTorrent handshake
func (pc *PeerConnection) handshake() error {
	// Handshake format:
	// 1 byte: protocol identifier length (19)
	// 19 bytes: "BitTorrent protocol"
	// 8 bytes: reserved (extensions)
	// 20 bytes: info_hash
	// 20 bytes: peer_id

	handshake := make([]byte, 68)
	handshake[0] = 19
	copy(handshake[1:20], "BitTorrent protocol")
	// handshake[20:28] = reserved (zeros)
	copy(handshake[28:48], pc.infoHash[:])
	copy(handshake[48:68], pc.peerID[:])

	// Send handshake
	pc.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if _, err := pc.conn.Write(handshake); err != nil {
		return fmt.Errorf("handshake write failed: %w", err)
	}

	// Receive handshake response
	response := make([]byte, 68)
	pc.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if _, err := io.ReadFull(pc.conn, response); err != nil {
		return fmt.Errorf("handshake read failed: %w", err)
	}

	// Validate handshake
	if response[0] != 19 {
		return errors.New("invalid protocol identifier length")
	}
	if string(response[1:20]) != "BitTorrent protocol" {
		return errors.New("invalid protocol identifier")
	}

	// Verify info hash
	var receivedHash [20]byte
	copy(receivedHash[:], response[28:48])
	if receivedHash != pc.infoHash {
		return errors.New("info hash mismatch")
	}

	// Extract remote peer ID
	copy(pc.remotePeerID[:], response[48:68])

	// Clear read deadline for normal operation
	pc.conn.SetReadDeadline(time.Time{})

	return nil
}

// ReadMessage reads a message from the peer
func (pc *PeerConnection) ReadMessage() (*PeerMessage, error) {
	// Read 4-byte length prefix
	var length uint32
	if err := binary.Read(pc.conn, binary.BigEndian, &length); err != nil {
		return nil, err
	}

	// Keep-alive message (length = 0)
	if length == 0 {
		return nil, nil
	}

	// Read message
	msgData := make([]byte, length)
	if _, err := io.ReadFull(pc.conn, msgData); err != nil {
		return nil, err
	}

	msg := &PeerMessage{
		ID:      msgData[0],
		Payload: msgData[1:],
	}

	return msg, nil
}

// SendMessage sends a message to the peer
func (pc *PeerConnection) SendMessage(msg *PeerMessage) error {
	var msgLen uint32
	if msg == nil {
		// Keep-alive
		msgLen = 0
	} else {
		msgLen = uint32(1 + len(msg.Payload))
	}

	// Write length prefix
	if err := binary.Write(pc.conn, binary.BigEndian, msgLen); err != nil {
		return err
	}

	if msg != nil {
		// Write message ID
		if err := binary.Write(pc.conn, binary.BigEndian, msg.ID); err != nil {
			return err
		}

		// Write payload
		if len(msg.Payload) > 0 {
			if _, err := pc.conn.Write(msg.Payload); err != nil {
				return err
			}
		}
	}

	return nil
}

// SendInterested sends an interested message
func (pc *PeerConnection) SendInterested() error {
	pc.amInterested = true
	return pc.SendMessage(&PeerMessage{ID: MsgInterested})
}

// SendNotInterested sends a not interested message
func (pc *PeerConnection) SendNotInterested() error {
	pc.amInterested = false
	return pc.SendMessage(&PeerMessage{ID: MsgNotInterested})
}

// SendUnchoke sends an unchoke message
func (pc *PeerConnection) SendUnchoke() error {
	pc.amChoking = false
	return pc.SendMessage(&PeerMessage{ID: MsgUnchoke})
}

// SendHave broadcasts that we have a piece
func (pc *PeerConnection) SendHave(pieceIndex uint32) error {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, pieceIndex)
	return pc.SendMessage(&PeerMessage{ID: MsgHave, Payload: payload})
}

// RequestBlock requests a 16KB block from a piece
func (pc *PeerConnection) RequestBlock(pieceIndex, begin, length uint32) error {
	payload := make([]byte, 12)
	binary.BigEndian.PutUint32(payload[0:4], pieceIndex)
	binary.BigEndian.PutUint32(payload[4:8], begin)
	binary.BigEndian.PutUint32(payload[8:12], length)
	return pc.SendMessage(&PeerMessage{ID: MsgRequest, Payload: payload})
}

// ParseBitfield parses a bitfield message
func (pc *PeerConnection) ParseBitfield(payload []byte, numPieces int) {
	pc.bitfield = make([]bool, numPieces)
	for i := 0; i < numPieces; i++ {
		byteIndex := i / 8
		bitIndex := uint(7 - (i % 8))
		if byteIndex < len(payload) {
			pc.bitfield[i] = (payload[byteIndex] & (1 << bitIndex)) != 0
		}
	}
}

// HasPiece returns whether the peer has a specific piece
func (pc *PeerConnection) HasPiece(index int) bool {
	if index < 0 || index >= len(pc.bitfield) {
		return false
	}
	return pc.bitfield[index]
}

// Close closes the connection
func (pc *PeerConnection) Close() error {
	return pc.conn.Close()
}

// GetAddr returns the peer's address
func (pc *PeerConnection) GetAddr() PeerAddr {
	return pc.addr
}
