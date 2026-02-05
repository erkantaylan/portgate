package main

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// DiscoveredPort represents a port found by the scanner or registered manually.
type DiscoveredPort struct {
	Port        int       `json:"port"`
	Protocol    string    `json:"protocol"`
	ServiceName string    `json:"serviceName"`
	Title       string    `json:"title"`
	Healthy     bool      `json:"healthy"`
	LastSeen    time.Time `json:"lastSeen"`
	Source      string    `json:"source"`  // "scan" or "manual"
	ExePath     string    `json:"exePath"` // filesystem path of the listening process
}

// ManualPort is a user-registered port persisted in config.
type ManualPort struct {
	Port int    `json:"port"`
	Name string `json:"name,omitempty"`
	Path string `json:"path,omitempty"` // optional user-specified install path
}

// ScanRange defines a range of ports to scan.
type ScanRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// DomainMapping maps a subdomain to a target port.
type DomainMapping struct {
	Domain     string    `json:"domain"`
	TargetPort int       `json:"targetPort"`
	CreatedAt  time.Time `json:"createdAt"`
	System     bool      `json:"system,omitempty"`
}

// Config is the persisted configuration.
type Config struct {
	Mappings        []DomainMapping `json:"mappings"`
	ScanIntervalSec int             `json:"scanIntervalSec"`
	ScanRanges      []ScanRange     `json:"scanRanges,omitempty"`
	ManualPorts     []ManualPort    `json:"manualPorts,omitempty"`
}

// PortRequest is the POST body for registering a manual port.
type PortRequest struct {
	Port int    `json:"port"`
	Name string `json:"name,omitempty"`
	Path string `json:"path,omitempty"`
}

// ScanRangeRequest is the POST body for adding/removing a scan range.
type ScanRangeRequest struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Hub coordinates scanner, proxy, config, and WebSocket clients.
type Hub struct {
	mu         sync.RWMutex
	ports      []DiscoveredPort
	config     *ConfigStore
	clients    map[*WSClient]bool
	register   chan *WSClient
	unregister chan *WSClient
	broadcast  chan []byte
}

// WSClient represents a connected WebSocket client.
type WSClient struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// WSMessage is the WebSocket message envelope.
type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// MappingRequest is the POST body for creating a mapping.
type MappingRequest struct {
	Domain string `json:"domain"`
	Port   int    `json:"port"`
}
