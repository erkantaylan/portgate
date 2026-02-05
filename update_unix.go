//go:build !windows

package main

import "os"

func selfReplace(currentPath, newPath string) error {
	if err := os.Chmod(newPath, 0755); err != nil {
		return err
	}
	return os.Rename(newPath, currentPath)
}
