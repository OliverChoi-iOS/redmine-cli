package cache

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type NamedID struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Identifier string `json:"identifier,omitempty"`
	Login      string `json:"login,omitempty"`
	Firstname  string `json:"firstname,omitempty"`
	Lastname   string `json:"lastname,omitempty"`
}

func (n NamedID) FullName() string {
	if n.Firstname == "" && n.Lastname == "" {
		return ""
	}
	if n.Firstname == "" {
		return n.Lastname
	}
	if n.Lastname == "" {
		return n.Firstname
	}
	return n.Firstname + " " + n.Lastname
}

type Metadata struct {
	Statuses     []NamedID `json:"statuses,omitempty"`
	Trackers     []NamedID `json:"trackers,omitempty"`
	Priorities   []NamedID `json:"priorities,omitempty"`
	CustomFields []NamedID `json:"custom_fields,omitempty"`
	Projects     []NamedID `json:"projects,omitempty"`
	Users        []NamedID `json:"users,omitempty"`
	SyncedAt     time.Time `json:"synced_at"`
}

func (m Metadata) Fresh(now time.Time, ttl time.Duration) bool {
	if m.SyncedAt.IsZero() || ttl <= 0 {
		return false
	}
	return now.Sub(m.SyncedAt) <= ttl
}

type Status struct {
	Metadata  string `json:"metadata"`
	Refreshed bool   `json:"refreshed"`
	Age       int64  `json:"age_seconds,omitempty"`
}

func MetadataPath(cacheDir string) string {
	return filepath.Join(cacheDir, "metadata.json")
}

func LoadMetadata(path string) (Metadata, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Metadata{}, false, nil
	}
	if err != nil {
		return Metadata{}, false, err
	}

	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return Metadata{}, true, err
	}
	return meta, true, nil
}

func SaveMetadata(path string, meta Metadata) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func StatusFor(meta Metadata, now time.Time, ttl time.Duration, loaded bool, refreshed bool, bypassed bool) Status {
	if bypassed {
		return Status{Metadata: "bypassed", Refreshed: refreshed}
	}
	if refreshed {
		return Status{Metadata: "refreshed", Refreshed: true}
	}
	if loaded && meta.Fresh(now, ttl) {
		return Status{Metadata: "hit", Age: int64(now.Sub(meta.SyncedAt).Seconds())}
	}
	return Status{Metadata: "miss"}
}
