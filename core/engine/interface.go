package engine

import "github.com/anacrolix/torrent"

type EngineInterface interface {
	Config() Config
	Configure(Config) error
	NewMagnet(string) error
	NewTorrent(*torrent.TorrentSpec) error
	GetTorrents() map[string]*Torrent
	StartTorrent(string) error
	StopTorrent(string) error
	DeleteTorrent(string) error
	StartFile(string, string) error
	StopFile(string, string) error
	AttachPersister(*Persister)
	DetachPersister()
	RehydrateFromPersister()
}
