//go:build windows

package main

import "os"

func selfReplace(currentPath, newPath string) error {
	oldPath := currentPath + ".old"
	os.Remove(oldPath) // remove leftover from previous update
	if err := os.Rename(currentPath, oldPath); err != nil {
		return err
	}
	if err := os.Rename(newPath, currentPath); err != nil {
		os.Rename(oldPath, currentPath) // try to restore
		return err
	}
	return nil
}
