package engine

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"os"
)

type MetaInfo struct {
	Announce     string     // Primary tracker URL
	AnnounceList [][]string // Tiered tracker list (BEP 12)
	Info         InfoDict   // The info dictionary
	InfoHash     [20]byte   // SHA-1 hash of bencoded info dict
	InfoBytes    []byte     // Raw bencoded info dict (for hash calculation)
}

type InfoDict struct {
	Name        string     // Suggested filename
	PieceLength int64      // Bytes per piece (typically 256KB or 512KB)
	Pieces      [][20]byte // SHA-1 hashes of each piece
	Length      int64      // For single-file torrents
	Files       []FileInfo // For multi-file torrents
}

type FileInfo struct {
	Path   []string // Path components
	Length int64    // File size in bytes
}

// ParseMetaInfo parses a .torrent file
func ParseMetaInfo(path string) (*MetaInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read torrent file: %w", err)
	}

	decoder := NewBencodeDecoder(data)
	root, err := decoder.Decode()
	if err != nil {
		return nil, fmt.Errorf("failed to decode torrent: %w", err)
	}

	rootDict, ok := root.(BencodeDict)
	if !ok {
		return nil, errors.New("root must be a dictionary")
	}

	metaInfo := &MetaInfo{}

	// Extract announce
	if announce, ok := rootDict["announce"].(BencodeString); ok {
		metaInfo.Announce = string(announce)
	}

	// Extract announce-list (BEP 12)
	if announceList, ok := rootDict["announce-list"].(BencodeList); ok {
		for _, tier := range announceList {
			if tierList, ok := tier.(BencodeList); ok {
				var tierURLs []string
				for _, url := range tierList {
					if urlStr, ok := url.(BencodeString); ok {
						tierURLs = append(tierURLs, string(urlStr))
					}
				}
				if len(tierURLs) > 0 {
					metaInfo.AnnounceList = append(metaInfo.AnnounceList, tierURLs)
				}
			}
		}
	}

	// Extract and hash the info dictionary
	infoVal, ok := rootDict["info"]
	if !ok {
		return nil, errors.New("missing info dictionary")
	}

	// Calculate info-hash from raw bencoded info dict
	metaInfo.InfoBytes = infoVal.Encode()
	hash := sha1.Sum(metaInfo.InfoBytes)
	metaInfo.InfoHash = hash

	// Parse info dictionary
	infoDict, ok := infoVal.(BencodeDict)
	if !ok {
		return nil, errors.New("info must be a dictionary")
	}

	if err := parseInfoDict(&metaInfo.Info, infoDict); err != nil {
		return nil, err
	}

	return metaInfo, nil
}

func parseInfoDict(info *InfoDict, dict BencodeDict) error {
	// Name
	if name, ok := dict["name"].(BencodeString); ok {
		info.Name = string(name)
	}

	// Piece length
	if pieceLength, ok := dict["piece length"].(BencodeInt); ok {
		info.PieceLength = int64(pieceLength)
	} else {
		return errors.New("missing piece length")
	}

	// Pieces (concatenated SHA-1 hashes)
	if piecesStr, ok := dict["pieces"].(BencodeString); ok {
		if len(piecesStr)%20 != 0 {
			return errors.New("pieces length must be multiple of 20")
		}
		numPieces := len(piecesStr) / 20
		info.Pieces = make([][20]byte, numPieces)
		for i := 0; i < numPieces; i++ {
			copy(info.Pieces[i][:], piecesStr[i*20:(i+1)*20])
		}
	} else {
		return errors.New("missing pieces")
	}

	// Single-file mode
	if length, ok := dict["length"].(BencodeInt); ok {
		info.Length = int64(length)
		return nil
	}

	// Multi-file mode
	if filesVal, ok := dict["files"].(BencodeList); ok {
		for _, fileVal := range filesVal {
			fileDict, ok := fileVal.(BencodeDict)
			if !ok {
				return errors.New("file entry must be dictionary")
			}

			var fileInfo FileInfo

			if length, ok := fileDict["length"].(BencodeInt); ok {
				fileInfo.Length = int64(length)
			}

			if pathList, ok := fileDict["path"].(BencodeList); ok {
				for _, pathPart := range pathList {
					if pathStr, ok := pathPart.(BencodeString); ok {
						fileInfo.Path = append(fileInfo.Path, string(pathStr))
					}
				}
			}

			info.Files = append(info.Files, fileInfo)
		}
		return nil
	}

	return errors.New("torrent must have either length or files")
}

// TotalLength returns total size of all files
func (m *MetaInfo) TotalLength() int64 {
	if m.Info.Length > 0 {
		return m.Info.Length
	}
	var total int64
	for _, f := range m.Info.Files {
		total += f.Length
	}
	return total
}

// NumPieces returns the number of pieces
func (m *MetaInfo) NumPieces() int {
	return len(m.Info.Pieces)
}
