package main

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// DiscoveredPort represents a port found by the scanner.
type DiscoveredPort struct {
	Port        int       `json:"port"`
	Protocol    string    `json:"protocol"`
	ServiceName string    `json:"serviceName"`
	Title       string    `json:"title"`
	Healthy     bool      `json:"healthy"`
	LastSeen    time.Time `json:"lastSeen"`
}

// DomainMapping maps a subdomain to a target port.
type DomainMapping struct {
	Domain     string    `json:"domain"`
	TargetPort int       `json:"targetPort"`
	CreatedAt  time.Time `json:"createdAt"`
}

// Config is the persisted configuration.
type Config struct {
	Mappings        []DomainMapping `json:"mappings"`
	ScanIntervalSec int            `json:"scanIntervalSec"`
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
