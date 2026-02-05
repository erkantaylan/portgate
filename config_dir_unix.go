//go:build !windows

package main

import (
	"os"
	"path/filepath"
)

func defaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "portgate", "config.json"), nil
}
