package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// ConfigStore handles loading and saving config to JSON.
type ConfigStore struct {
	mu   sync.RWMutex
	path string
	cfg  Config
}

// DefaultScanRanges are used when no custom ranges are configured.
var DefaultScanRanges = []ScanRange{
	{Start: 3000, End: 3999},
	{Start: 4000, End: 4099},
	{Start: 5000, End: 5999},
	{Start: 8000, End: 8999},
}

// NewConfigStore creates a ConfigStore using the given path.
// If path is empty, uses a platform-appropriate default location.
func NewConfigStore(path string) (*ConfigStore, error) {
	if path == "" {
		var err error
		path, err = defaultConfigPath()
		if err != nil {
			return nil, err
		}
	}
	cs := &ConfigStore{path: path, cfg: Config{ScanIntervalSec: 10}}
	if err := cs.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return cs, nil
}

func (cs *ConfigStore) load() error {
	data, err := os.ReadFile(cs.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &cs.cfg)
}

// Save writes the config atomically (write tmp + rename).
func (cs *ConfigStore) Save() error {
	cs.mu.RLock()
	data, err := json.MarshalIndent(cs.cfg, "", "  ")
	cs.mu.RUnlock()
	if err != nil {
		return err
	}
	dir := filepath.Dir(cs.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp := cs.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, cs.path)
}

// Mappings returns a copy of the current domain mappings.
func (cs *ConfigStore) Mappings() []DomainMapping {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	out := make([]DomainMapping, len(cs.cfg.Mappings))
	copy(out, cs.cfg.Mappings)
	return out
}

// AddMapping adds a domain mapping and persists.
func (cs *ConfigStore) AddMapping(m DomainMapping) error {
	cs.mu.Lock()
	// Remove existing mapping for same domain
	filtered := cs.cfg.Mappings[:0]
	for _, existing := range cs.cfg.Mappings {
		if existing.Domain != m.Domain {
			filtered = append(filtered, existing)
		}
	}
	cs.cfg.Mappings = append(filtered, m)
	cs.mu.Unlock()
	return cs.Save()
}

// RemoveMapping removes a domain mapping and persists.
func (cs *ConfigStore) RemoveMapping(domain string) error {
	cs.mu.Lock()
	filtered := cs.cfg.Mappings[:0]
	for _, existing := range cs.cfg.Mappings {
		if existing.Domain != domain {
			filtered = append(filtered, existing)
		}
	}
	cs.cfg.Mappings = filtered
	cs.mu.Unlock()
	return cs.Save()
}

// LookupPort returns the target port for a domain, or 0 if not found.
func (cs *ConfigStore) LookupPort(domain string) int {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	for _, m := range cs.cfg.Mappings {
		if m.Domain == domain {
			return m.TargetPort
		}
	}
	return 0
}

// ScanRanges returns the configured scan ranges, or defaults if none set.
func (cs *ConfigStore) ScanRanges() []ScanRange {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	if len(cs.cfg.ScanRanges) == 0 {
		return DefaultScanRanges
	}
	out := make([]ScanRange, len(cs.cfg.ScanRanges))
	copy(out, cs.cfg.ScanRanges)
	return out
}

// AddScanRange adds a scan range and persists.
func (cs *ConfigStore) AddScanRange(sr ScanRange) error {
	cs.mu.Lock()
	// Initialize from defaults if empty
	if len(cs.cfg.ScanRanges) == 0 {
		cs.cfg.ScanRanges = make([]ScanRange, len(DefaultScanRanges))
		copy(cs.cfg.ScanRanges, DefaultScanRanges)
	}
	// Avoid duplicates
	for _, existing := range cs.cfg.ScanRanges {
		if existing.Start == sr.Start && existing.End == sr.End {
			cs.mu.Unlock()
			return nil
		}
	}
	cs.cfg.ScanRanges = append(cs.cfg.ScanRanges, sr)
	cs.mu.Unlock()
	return cs.Save()
}

// RemoveScanRange removes a scan range and persists.
func (cs *ConfigStore) RemoveScanRange(sr ScanRange) error {
	cs.mu.Lock()
	// Initialize from defaults if empty
	if len(cs.cfg.ScanRanges) == 0 {
		cs.cfg.ScanRanges = make([]ScanRange, len(DefaultScanRanges))
		copy(cs.cfg.ScanRanges, DefaultScanRanges)
	}
	filtered := cs.cfg.ScanRanges[:0]
	for _, existing := range cs.cfg.ScanRanges {
		if existing.Start != sr.Start || existing.End != sr.End {
			filtered = append(filtered, existing)
		}
	}
	cs.cfg.ScanRanges = filtered
	cs.mu.Unlock()
	return cs.Save()
}

// ManualPorts returns a copy of the manual ports.
func (cs *ConfigStore) ManualPorts() []ManualPort {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	out := make([]ManualPort, len(cs.cfg.ManualPorts))
	copy(out, cs.cfg.ManualPorts)
	return out
}

// AddManualPort adds a manual port and persists.
func (cs *ConfigStore) AddManualPort(mp ManualPort) error {
	cs.mu.Lock()
	// Replace if same port exists
	filtered := cs.cfg.ManualPorts[:0]
	for _, existing := range cs.cfg.ManualPorts {
		if existing.Port != mp.Port {
			filtered = append(filtered, existing)
		}
	}
	cs.cfg.ManualPorts = append(filtered, mp)
	cs.mu.Unlock()
	return cs.Save()
}

// RemoveManualPort removes a manual port and persists.
func (cs *ConfigStore) RemoveManualPort(port int) error {
	cs.mu.Lock()
	filtered := cs.cfg.ManualPorts[:0]
	for _, existing := range cs.cfg.ManualPorts {
		if existing.Port != port {
			filtered = append(filtered, existing)
		}
	}
	cs.cfg.ManualPorts = filtered
	cs.mu.Unlock()
	return cs.Save()
}
