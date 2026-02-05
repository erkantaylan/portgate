//go:build windows

package main

import (
	"os"
	"path/filepath"
)

func defaultConfigPath() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	return filepath.Join(appData, "portgate", "config.json"), nil
}
