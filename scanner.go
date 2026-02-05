package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var scanRanges = [][2]int{
	{3000, 3999},
	{4000, 4099},
	{5000, 5999},
	{8000, 8999},
}

var titleRe = regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)

// Scanner scans TCP ports and detects HTTP services.
type Scanner struct {
	interval time.Duration
	onChange func([]DiscoveredPort)
}

// NewScanner creates a scanner with the given interval and change callback.
func NewScanner(interval time.Duration, onChange func([]DiscoveredPort)) *Scanner {
	return &Scanner{interval: interval, onChange: onChange}
}

// Run starts scanning in a loop until ctx is cancelled.
func (s *Scanner) Run(ctx context.Context) {
	// Initial scan immediately
	ports := s.scan()
	if s.onChange != nil {
		s.onChange(ports)
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ports := s.scan()
			if s.onChange != nil {
				s.onChange(ports)
			}
		}
	}
}

func (s *Scanner) scan() []DiscoveredPort {
	var ports []DiscoveredPort
	now := time.Now()

	for _, r := range scanRanges {
		for port := r[0]; port <= r[1]; port++ {
			if isOpen(port) {
				dp := DiscoveredPort{
					Port:     port,
					Protocol: "tcp",
					Healthy:  true,
					LastSeen: now,
				}
				s.probeHTTP(&dp)
				ports = append(ports, dp)
			}
		}
	}
	return ports
}

func isOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func (s *Scanner) probeHTTP(dp *DiscoveredPort) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/", dp.Port))
	if err != nil {
		dp.ServiceName = "tcp"
		return
	}
	defer resp.Body.Close()

	dp.ServiceName = "http"

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return
	}

	if matches := titleRe.FindSubmatch(body); len(matches) > 1 {
		dp.Title = strings.TrimSpace(string(matches[1]))
	}

	serverHeader := resp.Header.Get("Server")
	if serverHeader != "" && dp.Title == "" {
		dp.Title = serverHeader
	}
}
