package engine

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
)

type Engine struct {
	mut       sync.Mutex
	cacheDir  string
	client    *torrent.Client
	config    Config
	ts        map[string]*Torrent
	persister *Persister
	persistQ  chan persistOp
	persistWg *sync.WaitGroup
}

func New() *Engine {
	return &Engine{ts: map[string]*Torrent{}}
}

type persistOp struct {
	Op           string
	InfoHash     string
	Name         string
	Magnet       string
	TorrentPath  string
	DesiredState string
}

// AttachPersister attaches a Persister and starts a background worker
// to process persistence operations.
func (e *Engine) AttachPersister(p *Persister) {
	e.mut.Lock()
	defer e.mut.Unlock()
	if p == nil {
		return
	}
	e.persister = p
	if e.persistQ == nil {
		e.persistQ = make(chan persistOp, 128)
		e.persistWg = &sync.WaitGroup{}
		e.persistWg.Add(1)
		go func() {
			defer e.persistWg.Done()
			for op := range e.persistQ {
				switch op.Op {
				case "upsert":
					if e.persister != nil {
						_ = e.persister.UpsertTorrent(op.InfoHash, op.Name, op.Magnet, op.TorrentPath, op.DesiredState)
					}
				case "delete":
					if e.persister != nil {
						_ = e.persister.DeleteTorrent(op.InfoHash)
					}
				}
			}
		}()
	}
}

// DetachPersister gracefully shuts down the persistence worker and clears the persister.
func (e *Engine) DetachPersister() {
	e.mut.Lock()
	ch := e.persistQ
	wg := e.persistWg
	e.persistQ = nil
	e.persistWg = nil
	e.persister = nil
	e.mut.Unlock()
	if ch != nil {
		close(ch)
	}
	if wg != nil {
		wg.Wait()
	}
}

// RehydrateFromPersister loads persisted torrents and re-adds them to the engine.
func (e *Engine) RehydrateFromPersister() {
	e.mut.Lock()
	p := e.persister
	e.mut.Unlock()
	if p == nil {
		return
	}
	rows, err := p.GetAllTorrents()
	if err != nil {
		log.Printf("rehydrate: failed to read persisted torrents: %v", err)
		return
	}
	for _, r := range rows {
		magnet := r["magnet"]
		infohash := r["infohash"]
		desired := r["desired_state"]
		torrentPath := r["torrent_path"]
		if magnet != "" {
			// sanitize and add
			san, _, err := SanitizeMagnet(magnet)
			if err != nil {
				log.Printf("rehydrate: invalid magnet for %s: %v", infohash, err)
				continue
			}
			// directly add magnet and control desired start
			tt, err := e.client.AddMagnet(san)
			if err != nil {
				log.Printf("rehydrate: failed to add magnet %s: %v", infohash, err)
				continue
			}
			if err := e.newTorrent(tt, desired == "started"); err != nil {
				log.Printf("rehydrate: failed to register magnet %s: %v", infohash, err)
				continue
			}
			// proceed to next persisted row
			continue
		}
		// attempt to restore from a stored .torrent file path
		if torrentPath != "" {
			// Adding from a .torrent file is not implemented in rehydration yet.
			// Implementing this requires constructing a torrent spec from the
			// .torrent meta-info and calling client.AddTorrentSpec, which
			// depends on the anacrolix API. We'll skip for now and log.
			log.Printf("rehydrate: skipping torrent file restore for %s (path=%s)", infohash, torrentPath)
			continue
		}
		// TODO: support torrent_path restore
		_ = infohash
	}
}

func (e *Engine) enqueuePersist(op persistOp) {
	if e.persistQ == nil {
		return
	}
	select {
	case e.persistQ <- op:
	default:
		// drop if queue is full to avoid blocking
	}
}

func (e *Engine) Config() Config {
	return e.config
}

func (e *Engine) Configure(c Config) error {
	//recieve config
	if e.client != nil {
		e.client.Close()
		time.Sleep(1 * time.Second)
	}
	if c.IncomingPort <= 0 {
		return fmt.Errorf("Invalid incoming port (%d)", c.IncomingPort)
	}

	config := torrent.NewDefaultClientConfig()
	config.DataDir = c.DownloadDirectory
	config.NoUpload = !c.EnableUpload
	config.Seed = c.EnableSeeding
	config.ListenPort = c.IncomingPort
	client, err := torrent.NewClient(config)
	if err != nil {
		return err
	}
	e.mut.Lock()
	e.config = c
	e.client = client
	e.mut.Unlock()
	//reset
	e.GetTorrents()
	return nil
}

func (e *Engine) NewMagnet(magnetURI string) error {
	// defensive: validate magnet and sanitize trackers
	safe, err := sanitizeMagnet(magnetURI)
	if err != nil {
		return err
	}

	// recover from possible panics inside the client library
	defer func() error {
		if r := recover(); r != nil {
			return fmt.Errorf("panic in AddMagnet: %v", r)
		}
		return nil
	}()

	tt, err := e.client.AddMagnet(safe)
	if err != nil {
		return err
	}
	if err := e.newTorrent(tt, e.config.AutoStart); err != nil {
		return err
	}
	// persist metadata (magnet) if available
	if e.persister != nil {
		ih := tt.InfoHash().HexString()
		name := tt.Name()
		desired := "stopped"
		if e.config.AutoStart {
			desired = "started"
		}
		e.enqueuePersist(persistOp{Op: "upsert", InfoHash: ih, Name: name, Magnet: magnetURI, DesiredState: desired})
	}
	return nil
}

func (e *Engine) NewTorrent(spec *torrent.TorrentSpec) error {
	// recover from panics in underlying library
	defer func() error {
		if r := recover(); r != nil {
			return fmt.Errorf("panic in AddTorrentSpec: %v", r)
		}
		return nil
	}()

	tt, _, err := e.client.AddTorrentSpec(spec)
	if err != nil {
		return err
	}
	if err := e.newTorrent(tt, e.config.AutoStart); err != nil {
		return err
	}
	if e.persister != nil {
		ih := tt.InfoHash().HexString()
		name := tt.Name()
		desired := "stopped"
		if e.config.AutoStart {
			desired = "started"
		}
		e.enqueuePersist(persistOp{Op: "upsert", InfoHash: ih, Name: name, TorrentPath: "", DesiredState: desired})
	}
	return nil
}

// sanitizeMagnet removes invalid trackers and validates the magnet URI.
// It returns a possibly modified magnet URI or an error if the input is invalid.
func sanitizeMagnet(m string) (string, error) {
	if strings.TrimSpace(m) == "" {
		return "", errors.New("empty magnet URI")
	}
	if !strings.HasPrefix(m, "magnet:") {
		return "", errors.New("invalid magnet URI: missing 'magnet:' scheme")
	}
	u, err := url.Parse(m)
	if err != nil {
		return "", fmt.Errorf("invalid magnet URI: %w", err)
	}
	// Ensure xt contains urn:btih
	q := u.Query()
	xts := q["xt"]
	if len(xts) == 0 {
		return "", errors.New("magnet URI missing xt parameter")
	}
	// Sanitize trackers: remove trackers with empty or unknown schemes
	goodTr := []string{}
	for _, tr := range q["tr"] {
		tu, err := url.Parse(tr)
		if err != nil || tu.Scheme == "" {
			// skip invalid tracker
			continue
		}
		// keep only http(s) and udp schemes commonly used by trackers
		switch strings.ToLower(tu.Scheme) {
		case "http", "https", "udp":
			goodTr = append(goodTr, tr)
		default:
			// skip unknown scheme
		}
	}
	// Rebuild query with sanitized trackers
	newQ := url.Values{}
	for _, xt := range xts {
		newQ.Add("xt", xt)
	}
	if dn := q.Get("dn"); dn != "" {
		newQ.Set("dn", dn)
	}
	for _, tr := range goodTr {
		newQ.Add("tr", tr)
	}
	u.RawQuery = newQ.Encode()
	return u.String(), nil
}

// SanitizeMagnet is an exported wrapper that returns the sanitized magnet URI
// along with a list of dropped trackers (for user-facing warnings).
func SanitizeMagnet(m string) (string, []string, error) {
	if strings.TrimSpace(m) == "" {
		return "", nil, errors.New("empty magnet URI")
	}
	if !strings.HasPrefix(m, "magnet:") {
		return "", nil, errors.New("invalid magnet URI: missing 'magnet:' scheme")
	}
	u, err := url.Parse(m)
	if err != nil {
		return "", nil, fmt.Errorf("invalid magnet URI: %w", err)
	}
	q := u.Query()
	if len(q["xt"]) == 0 {
		return "", nil, errors.New("magnet URI missing xt parameter")
	}
	goodTr := []string{}
	dropped := []string{}
	for _, tr := range q["tr"] {
		tu, err := url.Parse(tr)
		if err != nil || tu.Scheme == "" {
			dropped = append(dropped, tr)
			continue
		}
		switch strings.ToLower(tu.Scheme) {
		case "http", "https", "udp":
			goodTr = append(goodTr, tr)
		default:
			dropped = append(dropped, tr)
		}
	}
	newQ := url.Values{}
	for _, xt := range q["xt"] {
		newQ.Add("xt", xt)
	}
	if dn := q.Get("dn"); dn != "" {
		newQ.Set("dn", dn)
	}
	for _, tr := range goodTr {
		newQ.Add("tr", tr)
	}
	u.RawQuery = newQ.Encode()
	return u.String(), dropped, nil
}

func (e *Engine) newTorrent(tt *torrent.Torrent, desiredStart bool) error {
	t := e.upsertTorrent(tt)
	go func() {
		<-t.t.GotInfo()
		if desiredStart || e.config.AutoStart {
			e.StartTorrent(t.InfoHash)
		}
	}()
	return nil
}

func (e *Engine) GetTorrents() map[string]*Torrent {
	e.mut.Lock()
	defer e.mut.Unlock()

	if e.client == nil {
		return nil
	}
	for _, tt := range e.client.Torrents() {
		e.upsertTorrent(tt)
	}
	return e.ts
}

func (e *Engine) upsertTorrent(tt *torrent.Torrent) *Torrent {
	ih := tt.InfoHash().HexString()
	torrent, ok := e.ts[ih]
	if !ok {
		torrent = &Torrent{InfoHash: ih}
		e.ts[ih] = torrent
	}
	//update torrent fields using underlying torrent
	torrent.Update(tt)
	// Persist new/updated torrent metadata asynchronously
	if e.persister != nil {
		desired := "stopped"
		if torrent.Started {
			desired = "started"
		}
		e.enqueuePersist(persistOp{Op: "upsert", InfoHash: torrent.InfoHash, Name: torrent.Name, DesiredState: desired})
	}
	return torrent
}

func (e *Engine) getTorrent(infohash string) (*Torrent, error) {
	ih, err := str2ih(infohash)
	if err != nil {
		return nil, err
	}
	t, ok := e.ts[ih.HexString()]
	if !ok {
		return t, fmt.Errorf("Missing torrent %x", ih)
	}
	return t, nil
}

func (e *Engine) getOpenTorrent(infohash string) (*Torrent, error) {
	t, err := e.getTorrent(infohash)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (e *Engine) StartTorrent(infohash string) error {
	t, err := e.getOpenTorrent(infohash)
	if err != nil {
		return err
	}
	if t.Started {
		return fmt.Errorf("Already started")
	}
	t.Started = true
	for _, f := range t.Files {
		if f != nil {
			f.Started = true
		}
	}
	if t.t.Info() != nil {
		t.t.DownloadAll()
	}
	return nil
}

func (e *Engine) StopTorrent(infohash string) error {
	t, err := e.getTorrent(infohash)
	if err != nil {
		return err
	}
	if !t.Started {
		return fmt.Errorf("Already stopped")
	}
	//there is no stop - kill underlying torrent
	t.t.Drop()
	t.Started = false
	for _, f := range t.Files {
		if f != nil {
			f.Started = false
		}
	}
	return nil
}

func (e *Engine) DeleteTorrent(infohash string) error {
	t, err := e.getTorrent(infohash)
	if err != nil {
		return err
	}
	os.Remove(filepath.Join(e.cacheDir, infohash+".torrent"))
	delete(e.ts, t.InfoHash)
	ih, _ := str2ih(infohash)
	if tt, ok := e.client.Torrent(ih); ok {
		tt.Drop()
	}
	if e.persister != nil {
		e.enqueuePersist(persistOp{Op: "delete", InfoHash: t.InfoHash})
	}
	return nil
}

func (e *Engine) StartFile(infohash, filepath string) error {
	t, err := e.getOpenTorrent(infohash)
	if err != nil {
		return err
	}
	var f *File
	for _, file := range t.Files {
		if file.Path == filepath {
			f = file
			break
		}
	}
	if f == nil {
		return fmt.Errorf("Missing file %s", filepath)
	}
	if f.Started {
		return fmt.Errorf("Already started")
	}
	t.Started = true
	f.Started = true
	return nil
}

func (e *Engine) StopFile(infohash, filepath string) error {
	return fmt.Errorf("Unsupported")
}

func str2ih(str string) (metainfo.Hash, error) {
	var ih metainfo.Hash
	e, err := hex.Decode(ih[:], []byte(str))
	if err != nil {
		return ih, fmt.Errorf("Invalid hex string")
	}
	if e != 20 {
		return ih, fmt.Errorf("Invalid length")
	}
	return ih, nil
}
