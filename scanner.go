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

var titleRe = regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)

// Scanner scans TCP ports and detects HTTP services.
type Scanner struct {
	interval time.Duration
	config   *ConfigStore
	onChange func([]DiscoveredPort)
}

// NewScanner creates a scanner with the given interval, config store, and change callback.
func NewScanner(interval time.Duration, config *ConfigStore, onChange func([]DiscoveredPort)) *Scanner {
	return &Scanner{interval: interval, config: config, onChange: onChange}
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

	// Track which ports were found by scanning so we can mark manual ports correctly
	scannedPorts := make(map[int]bool)

	// Scan configurable ranges
	ranges := s.config.ScanRanges()
	for _, r := range ranges {
		for port := r.Start; port <= r.End; port++ {
			if isOpen(port) {
				dp := DiscoveredPort{
					Port:     port,
					Protocol: "tcp",
					Healthy:  true,
					LastSeen: now,
					Source:   "scan",
				}
				s.probeHTTP(&dp)
				ports = append(ports, dp)
				scannedPorts[port] = true
			}
		}
	}

	// Add manual ports — health-check each one
	for _, mp := range s.config.ManualPorts() {
		if scannedPorts[mp.Port] {
			// Already found by scan — update the source to show both, keep as scan
			// but ensure we don't duplicate; the scan result already has it
			continue
		}
		dp := DiscoveredPort{
			Port:     mp.Port,
			Protocol: "tcp",
			Healthy:  isOpen(mp.Port),
			LastSeen: now,
			Source:   "manual",
		}
		if mp.Name != "" {
			dp.Title = mp.Name
		}
		if dp.Healthy {
			s.probeHTTP(&dp)
			// Preserve manual name if probeHTTP didn't find a title
			if dp.Title == "" && mp.Name != "" {
				dp.Title = mp.Name
			}
		}
		ports = append(ports, dp)
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
