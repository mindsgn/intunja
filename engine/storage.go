package engine

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// StorageManager handles disk I/O with sparse files and write aggregation
type StorageManager struct {
	metaInfo     *MetaInfo
	downloadPath string

	// File handles
	files []*os.File

	// Write aggregation buffer
	writeBuffer map[int][]byte // pieceIndex -> data
	bufferMu    sync.Mutex

	// Piece verification cache
	pieceCache map[int][]byte
	cacheMu    sync.RWMutex
}

// NewStorageManager creates a storage manager
func NewStorageManager(metaInfo *MetaInfo, downloadPath string) (*StorageManager, error) {
	sm := &StorageManager{
		metaInfo:     metaInfo,
		downloadPath: downloadPath,
		writeBuffer:  make(map[int][]byte),
		pieceCache:   make(map[int][]byte),
	}

	// Create download directory
	if err := os.MkdirAll(downloadPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create download directory: %w", err)
	}

	// Allocate sparse files
	if err := sm.allocateFiles(); err != nil {
		return nil, err
	}

	return sm, nil
}

// allocateFiles creates sparse files for the download
func (sm *StorageManager) allocateFiles() error {
	if sm.metaInfo.Info.Length > 0 {
		// Single-file mode
		return sm.allocateSingleFile()
	}

	// Multi-file mode
	return sm.allocateMultiFile()
}

// allocateSingleFile creates a sparse file for single-file torrents
func (sm *StorageManager) allocateSingleFile() error {
	filePath := filepath.Join(sm.downloadPath, sm.metaInfo.Info.Name)

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	// Allocate sparse file using truncate
	if err := file.Truncate(sm.metaInfo.Info.Length); err != nil {
		file.Close()
		return fmt.Errorf("failed to allocate file: %w", err)
	}

	sm.files = []*os.File{file}
	return nil
}

// allocateMultiFile creates sparse files for multi-file torrents
func (sm *StorageManager) allocateMultiFile() error {
	baseDir := filepath.Join(sm.downloadPath, sm.metaInfo.Info.Name)

	for _, fileInfo := range sm.metaInfo.Info.Files {
		// Build file path
		pathParts := append([]string{baseDir}, fileInfo.Path...)
		filePath := filepath.Join(pathParts...)

		// Create directory structure
		dirPath := filepath.Dir(filePath)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}

		// Create sparse file
		file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}

		if err := file.Truncate(fileInfo.Length); err != nil {
			file.Close()
			return fmt.Errorf("failed to allocate file: %w", err)
		}

		sm.files = append(sm.files, file)
	}

	return nil
}

// WritePiece writes a piece to disk with buffering
func (sm *StorageManager) WritePiece(pieceIndex int, data []byte) error {
	// Add to write buffer
	sm.bufferMu.Lock()
	sm.writeBuffer[pieceIndex] = data
	shouldFlush := len(sm.writeBuffer) >= 10 // Flush every 10 pieces
	sm.bufferMu.Unlock()

	// Cache the piece for serving to other peers
	sm.cacheMu.Lock()
	sm.pieceCache[pieceIndex] = data
	sm.cacheMu.Unlock()

	// Flush if buffer is full
	if shouldFlush {
		return sm.FlushBuffer()
	}

	return nil
}

// FlushBuffer writes all buffered pieces to disk
func (sm *StorageManager) FlushBuffer() error {
	sm.bufferMu.Lock()
	defer sm.bufferMu.Unlock()

	for pieceIndex, data := range sm.writeBuffer {
		if err := sm.writePieceToDisk(pieceIndex, data); err != nil {
			return err
		}
		delete(sm.writeBuffer, pieceIndex)
	}

	return nil
}

// writePieceToDisk writes a single piece to the appropriate file(s)
func (sm *StorageManager) writePieceToDisk(pieceIndex int, data []byte) error {
	pieceLength := int64(sm.metaInfo.Info.PieceLength)
	pieceOffset := int64(pieceIndex) * pieceLength

	if sm.metaInfo.Info.Length > 0 {
		// Single-file mode: simple seek and write
		_, err := sm.files[0].WriteAt(data, pieceOffset)
		return err
	}

	// Multi-file mode: piece may span multiple files
	return sm.writeToMultiFile(pieceOffset, data)
}

// writeToMultiFile handles writing data that may span multiple files
func (sm *StorageManager) writeToMultiFile(offset int64, data []byte) error {
	var currentOffset int64
	remaining := data

	for fileIndex, fileInfo := range sm.metaInfo.Info.Files {
		fileEnd := currentOffset + fileInfo.Length

		if offset < fileEnd {
			// This file contains part of the data
			fileOffset := offset - currentOffset
			writeLen := fileInfo.Length - fileOffset

			if int64(len(remaining)) < writeLen {
				writeLen = int64(len(remaining))
			}

			_, err := sm.files[fileIndex].WriteAt(remaining[:writeLen], fileOffset)
			if err != nil {
				return err
			}

			remaining = remaining[writeLen:]
			offset += writeLen

			if len(remaining) == 0 {
				break
			}
		}

		currentOffset = fileEnd
	}

	return nil
}

// ReadPiece reads a piece from disk or cache
func (sm *StorageManager) ReadPiece(pieceIndex int) ([]byte, error) {
	// Check cache first
	sm.cacheMu.RLock()
	if cached, ok := sm.pieceCache[pieceIndex]; ok {
		sm.cacheMu.RUnlock()
		return cached, nil
	}
	sm.cacheMu.RUnlock()

	// Read from disk
	pieceLength := sm.calculatePieceLength(pieceIndex)
	pieceOffset := int64(pieceIndex) * int64(sm.metaInfo.Info.PieceLength)

	data := make([]byte, pieceLength)

	if sm.metaInfo.Info.Length > 0 {
		// Single-file mode
		_, err := sm.files[0].ReadAt(data, pieceOffset)
		if err != nil && err != io.EOF {
			return nil, err
		}
	} else {
		// Multi-file mode
		if err := sm.readFromMultiFile(pieceOffset, data); err != nil {
			return nil, err
		}
	}

	// Verify hash
	hash := sha1.Sum(data)
	if hash != sm.metaInfo.Info.Pieces[pieceIndex] {
		return nil, fmt.Errorf("piece %d hash verification failed", pieceIndex)
	}

	return data, nil
}

// readFromMultiFile handles reading data that may span multiple files
func (sm *StorageManager) readFromMultiFile(offset int64, data []byte) error {
	var currentOffset int64
	remaining := data

	for fileIndex, fileInfo := range sm.metaInfo.Info.Files {
		fileEnd := currentOffset + fileInfo.Length

		if offset < fileEnd {
			fileOffset := offset - currentOffset
			readLen := fileInfo.Length - fileOffset

			if int64(len(remaining)) < readLen {
				readLen = int64(len(remaining))
			}

			_, err := sm.files[fileIndex].ReadAt(remaining[:readLen], fileOffset)
			if err != nil && err != io.EOF {
				return err
			}

			remaining = remaining[readLen:]
			offset += readLen

			if len(remaining) == 0 {
				break
			}
		}

		currentOffset = fileEnd
	}

	return nil
}

// calculatePieceLength returns the length of a specific piece
func (sm *StorageManager) calculatePieceLength(index int) int {
	totalLength := sm.metaInfo.TotalLength()
	pieceLength := sm.metaInfo.Info.PieceLength

	if int64(index+1)*pieceLength > totalLength {
		return int(totalLength - int64(index)*pieceLength)
	}

	return int(pieceLength)
}

// Close closes all file handles and flushes buffers
func (sm *StorageManager) Close() error {
	// Flush any remaining buffered data
	if err := sm.FlushBuffer(); err != nil {
		return err
	}

	// Close all files
	for _, file := range sm.files {
		if err := file.Close(); err != nil {
			return err
		}
	}

	return nil
}
