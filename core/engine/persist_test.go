package engine

import (
	"testing"
)

func TestPersisterUpsertAndGet(t *testing.T) {
	p, err := NewPersister(":memory:")
	if err != nil {
		t.Fatalf("failed to open persister: %v", err)
	}
	defer p.Close()

	if err := p.UpsertTorrent("ih1", "name1", "magnet:?xt=urn:btih:abc", "", "started"); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	list, err := p.GetAllTorrents()
	if err != nil {
		t.Fatalf("get all torrents failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 torrent, got %d", len(list))
	}
	if list[0]["infohash"] != "ih1" {
		t.Fatalf("unexpected infohash: %s", list[0]["infohash"])
	}
}
