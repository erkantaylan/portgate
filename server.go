package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

//go:embed static
var staticFS embed.FS

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// NewHub creates a new Hub with the given config store.
func NewHub(cs *ConfigStore) *Hub {
	return &Hub{
		config:     cs,
		clients:    make(map[*WSClient]bool),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
		broadcast:  make(chan []byte, 256),
	}
}

// Run starts the Hub's client management loop.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case msg := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- msg:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

// SetPorts updates the discovered ports and broadcasts to clients.
func (h *Hub) SetPorts(ports []DiscoveredPort) {
	h.mu.Lock()
	h.ports = ports
	h.mu.Unlock()
	h.broadcastUpdate()
}

// GetPorts returns the current discovered ports.
func (h *Hub) GetPorts() []DiscoveredPort {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]DiscoveredPort, len(h.ports))
	copy(out, h.ports)
	return out
}

func (h *Hub) broadcastUpdate() {
	msg := struct {
		Ports    []DiscoveredPort `json:"ports"`
		Mappings []DomainMapping  `json:"mappings"`
	}{
		Ports:    h.GetPorts(),
		Mappings: h.config.Mappings(),
	}
	data, err := json.Marshal(WSMessage{Type: "update", Data: msg})
	if err != nil {
		return
	}
	h.broadcast <- data
}

// DashboardHandler returns the HTTP mux for the dashboard + API.
func DashboardHandler(hub *Hub) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/ports", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(hub.GetPorts())

		case http.MethodPost:
			var req PortRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			if req.Port < 1 || req.Port > 65535 {
				http.Error(w, "port must be 1-65535", http.StatusBadRequest)
				return
			}
			mp := ManualPort{Port: req.Port, Name: req.Name, Path: req.Path}
			if err := hub.config.AddManualPort(mp); err != nil {
				http.Error(w, "save failed", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(mp)

		case http.MethodDelete:
			portStr := r.URL.Query().Get("port")
			if portStr == "" {
				http.Error(w, "port required", http.StatusBadRequest)
				return
			}
			var port int
			if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
				http.Error(w, "invalid port", http.StatusBadRequest)
				return
			}
			if err := hub.config.RemoveManualPort(port); err != nil {
				http.Error(w, "save failed", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/scan-ranges", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(hub.config.ScanRanges())

		case http.MethodPost:
			var req ScanRangeRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			if req.Start < 1 || req.End > 65535 || req.Start > req.End {
				http.Error(w, "invalid range", http.StatusBadRequest)
				return
			}
			sr := ScanRange{Start: req.Start, End: req.End}
			if err := hub.config.AddScanRange(sr); err != nil {
				http.Error(w, "save failed", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(sr)

		case http.MethodDelete:
			startStr := r.URL.Query().Get("start")
			endStr := r.URL.Query().Get("end")
			if startStr == "" || endStr == "" {
				http.Error(w, "start and end required", http.StatusBadRequest)
				return
			}
			var start, end int
			if _, err := fmt.Sscanf(startStr, "%d", &start); err != nil {
				http.Error(w, "invalid start", http.StatusBadRequest)
				return
			}
			if _, err := fmt.Sscanf(endStr, "%d", &end); err != nil {
				http.Error(w, "invalid end", http.StatusBadRequest)
				return
			}
			sr := ScanRange{Start: start, End: end}
			if err := hub.config.RemoveScanRange(sr); err != nil {
				http.Error(w, "save failed", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/mappings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(hub.config.Mappings())

		case http.MethodPost:
			var req MappingRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			if req.Domain == "" || req.Port == 0 {
				http.Error(w, "domain and port required", http.StatusBadRequest)
				return
			}
			domain := strings.ToLower(strings.TrimSpace(req.Domain))
			domain = strings.TrimSuffix(domain, ".localhost")
			if domain == "portgate" || domain == "" {
				http.Error(w, "reserved domain", http.StatusBadRequest)
				return
			}
			m := DomainMapping{
				Domain:     domain,
				TargetPort: req.Port,
				CreatedAt:  time.Now(),
			}
			if err := hub.config.AddMapping(m); err != nil {
				http.Error(w, "save failed", http.StatusInternalServerError)
				return
			}
			hub.broadcastUpdate()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(m)

		case http.MethodDelete:
			domain := r.URL.Query().Get("domain")
			if domain == "" {
				http.Error(w, "domain required", http.StatusBadRequest)
				return
			}
			for _, m := range hub.config.Mappings() {
				if m.Domain == domain && m.System {
					http.Error(w, "cannot delete system mapping", http.StatusForbidden)
					return
				}
			}
			if err := hub.config.RemoveMapping(domain); err != nil {
				http.Error(w, "save failed", http.StatusInternalServerError)
				return
			}
			hub.broadcastUpdate()
			w.WriteHeader(http.StatusNoContent)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("ws upgrade error: %v", err)
			return
		}
		client := &WSClient{hub: hub, conn: conn, send: make(chan []byte, 256)}
		hub.register <- client

		go client.writePump()
		go client.readPump()

		// Send initial state
		msg := struct {
			Ports    []DiscoveredPort `json:"ports"`
			Mappings []DomainMapping  `json:"mappings"`
		}{
			Ports:    hub.GetPorts(),
			Mappings: hub.config.Mappings(),
		}
		data, _ := json.Marshal(WSMessage{Type: "update", Data: msg})
		client.send <- data
	})

	staticSub, _ := fs.Sub(staticFS, "static")
	mux.Handle("/", http.FileServer(http.FS(staticSub)))

	return mux
}

func (c *WSClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (c *WSClient) writePump() {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			break
		}
	}
}
