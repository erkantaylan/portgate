//go:build !windows

package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// findExeByPort returns the executable path of the process listening on the given TCP port.
// It reads /proc/net/tcp and /proc/net/tcp6 to find the socket inode, then walks
// /proc/*/fd/ to find the owning PID, and resolves /proc/<pid>/exe.
func findExeByPort(port int) string {
	inode := findSocketInode(port)
	if inode == "" {
		return ""
	}
	pid := findPIDByInode(inode)
	if pid == "" {
		return ""
	}
	exe, err := os.Readlink(filepath.Join("/proc", pid, "exe"))
	if err != nil {
		return ""
	}
	// Ignore deleted binaries marker
	exe = strings.TrimSuffix(exe, " (deleted)")
	return exe
}

// findSocketInode searches /proc/net/tcp and /proc/net/tcp6 for a LISTEN socket
// on the given port and returns its inode number as a string.
func findSocketInode(port int) string {
	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		if inode := findInodeInFile(path, port); inode != "" {
			return inode
		}
	}
	return ""
}

func findInodeInFile(path string, port int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if i == 0 { // skip header
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}
		// State 0A = LISTEN
		if fields[3] != "0A" {
			continue
		}
		// local_address is IP:PORT in hex
		parts := strings.SplitN(fields[1], ":", 2)
		if len(parts) != 2 {
			continue
		}
		portHex := parts[1]
		portBytes, err := hex.DecodeString(portHex)
		if err != nil || len(portBytes) != 2 {
			continue
		}
		localPort := int(portBytes[0])<<8 | int(portBytes[1])
		if localPort == port {
			return fields[9] // inode
		}
	}
	return ""
}

// findPIDByInode walks /proc/*/fd/ looking for a symlink to socket:[inode].
func findPIDByInode(inode string) string {
	target := fmt.Sprintf("socket:[%s]", inode)
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Only numeric directory names (PIDs)
		if _, err := strconv.Atoi(e.Name()); err != nil {
			continue
		}
		fdDir := filepath.Join("/proc", e.Name(), "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}
		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}
			if link == target {
				return e.Name()
			}
		}
	}
	return ""
}
