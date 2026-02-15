package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/anacrolix/torrent"
)

type RemoteEngine struct {
	baseURL    string
	httpClient *http.Client
}

func NewRemoteEngine(baseURL string) *RemoteEngine {
	return &RemoteEngine{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (r *RemoteEngine) Config() Config {
	return Config{}
}

func (r *RemoteEngine) Configure(c Config) error {
	b, _ := json.Marshal(&c)
	resp, err := r.httpClient.Post(r.baseURL+"/api/configure", "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("configure failed: %s", string(data))
	}
	return nil
}

func (r *RemoteEngine) NewMagnet(magnetURI string) error {
	resp, err := r.httpClient.Post(r.baseURL+"/api/magnet", "text/plain", bytes.NewReader([]byte(magnetURI)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("magnet failed: %s", string(data))
	}
	return nil
}

func (r *RemoteEngine) NewTorrent(spec *torrent.TorrentSpec) error {
	return fmt.Errorf("NewTorrent not implemented for remote engine")
}

func (r *RemoteEngine) GetTorrents() map[string]*Torrent {
	resp, err := r.httpClient.Get(r.baseURL + "/api/torrents")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	var ts map[string]*Torrent
	if err := json.Unmarshal(data, &ts); err != nil {
		return nil
	}
	return ts
}

func (r *RemoteEngine) StartTorrent(infohash string) error {
	body := []byte("start:" + infohash)
	resp, err := r.httpClient.Post(r.baseURL+"/api/torrent", "text/plain", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("start failed: %s", string(data))
	}
	return nil
}

func (r *RemoteEngine) StopTorrent(infohash string) error {
	body := []byte("stop:" + infohash)
	resp, err := r.httpClient.Post(r.baseURL+"/api/torrent", "text/plain", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("stop failed: %s", string(data))
	}
	return nil
}

func (r *RemoteEngine) DeleteTorrent(infohash string) error {
	body := []byte("delete:" + infohash)
	resp, err := r.httpClient.Post(r.baseURL+"/api/torrent", "text/plain", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("delete failed: %s", string(data))
	}
	return nil
}

func (r *RemoteEngine) StartFile(infohash, filepath string) error {
	body := []byte("start:" + infohash + ":" + filepath)
	resp, err := r.httpClient.Post(r.baseURL+"/api/file", "text/plain", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("start file failed: %s", string(data))
	}
	return nil
}

func (r *RemoteEngine) StopFile(infohash, filepath string) error {
	body := []byte("stop:" + infohash + ":" + filepath)
	resp, err := r.httpClient.Post(r.baseURL+"/api/file", "text/plain", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("stop file failed: %s", string(data))
	}
	return nil
}

// AttachPersister is a no-op for RemoteEngine (persistence handled by daemon)
func (r *RemoteEngine) AttachPersister(p *Persister) {}

func (r *RemoteEngine) DetachPersister() {}

func (r *RemoteEngine) RehydrateFromPersister() {}
