//go:build windows

package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// findExeByPort returns the executable path of the process listening on the given TCP port.
// It uses netstat to find the PID and then queries the Windows API for the process path.
func findExeByPort(port int) string {
	pid := findPIDByPort(port)
	if pid == 0 {
		return ""
	}
	return getProcessExePath(pid)
}

// findPIDByPort runs netstat -ano and finds the PID for a LISTENING socket on the given port.
func findPIDByPort(port int) int {
	out, err := exec.Command("netstat", "-ano").Output()
	if err != nil {
		return 0
	}
	needle := fmt.Sprintf(":%d ", port)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "LISTENING") {
			continue
		}
		if !strings.Contains(line, needle) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		// Verify the local address column matches our port exactly
		addr := fields[1]
		parts := strings.Split(addr, ":")
		if len(parts) < 2 {
			continue
		}
		p, err := strconv.Atoi(parts[len(parts)-1])
		if err != nil || p != port {
			continue
		}
		pid, err := strconv.Atoi(fields[len(fields)-1])
		if err != nil {
			continue
		}
		return pid
	}
	return 0
}

var (
	modKernel32              = syscall.NewLazyDLL("kernel32.dll")
	procQueryFullProcessName = modKernel32.NewProc("QueryFullProcessImageNameW")
)

// getProcessExePath returns the full image path for the given PID using the Windows API.
func getProcessExePath(pid int) string {
	const PROCESS_QUERY_LIMITED_INFORMATION = 0x1000

	handle, err := syscall.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return ""
	}
	defer syscall.CloseHandle(handle)

	buf := make([]uint16, 1024)
	size := uint32(len(buf))
	ret, _, _ := procQueryFullProcessName.Call(
		uintptr(handle),
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ret == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:size])
}
