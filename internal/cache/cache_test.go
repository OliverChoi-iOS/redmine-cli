package cache

import (
	"testing"
	"time"
)

func TestMetadataFresh(t *testing.T) {
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	meta := Metadata{SyncedAt: now.Add(-time.Hour)}

	if !meta.Fresh(now, 24*time.Hour) {
		t.Fatalf("metadata should be fresh")
	}
	if meta.Fresh(now, 30*time.Minute) {
		t.Fatalf("metadata should be stale")
	}
}
