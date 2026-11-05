//go:build !windows

package config

import (
	"fmt"
	"os"
)

func validateConfigDirPermissions(_ string, info os.FileInfo) error {
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("config parent directory must not be group- or world-writable")
	}
	return nil
}

func secureConfigFile(path string) error {
	return os.Chmod(path, 0o600)
}

func createPrivateTemp(dir string) (*os.File, error) {
	f, err := os.CreateTemp(dir, ".config.toml.*.tmp")
	if err != nil {
		return nil, err
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return nil, err
	}
	return f, nil
}
