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
