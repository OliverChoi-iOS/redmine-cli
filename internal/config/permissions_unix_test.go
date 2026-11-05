//go:build !windows

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func assertPrivateConfigFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}
}

func TestWriteFileRejectsGroupWritableParent(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o770); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(filepath.Join(dir, "config.toml"), Defaults()); err == nil {
		t.Fatal("WriteFile accepted a group-writable directory")
	}
}
