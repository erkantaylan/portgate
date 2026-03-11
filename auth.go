package main

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const sessionCookieName = "portgate_session"

// SessionStore manages auth sessions in memory.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]time.Time // token → expiry
}

// NewSessionStore creates a new session store.
func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]time.Time)}
}

// Create generates a new session token valid for the given duration.
func (ss *SessionStore) Create(expiry time.Duration) string {
	b := make([]byte, 32)
	rand.Read(b)
	token := hex.EncodeToString(b)

	ss.mu.Lock()
	ss.sessions[token] = time.Now().Add(expiry)
	ss.mu.Unlock()

	return token
}

// Valid checks whether a session token is valid.
func (ss *SessionStore) Valid(token string) bool {
	ss.mu.RLock()
	exp, ok := ss.sessions[token]
	ss.mu.RUnlock()
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		ss.mu.Lock()
		delete(ss.sessions, token)
		ss.mu.Unlock()
		return false
	}
	return true
}

// Cleanup removes expired sessions.
func (ss *SessionStore) Cleanup() {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	now := time.Now()
	for token, exp := range ss.sessions {
		if now.After(exp) {
			delete(ss.sessions, token)
		}
	}
}

// CheckPassword verifies a plaintext password against the stored bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// HashPassword creates a bcrypt hash from a plaintext password.
func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// isLocalRequest checks if the request originates from localhost.
func isLocalRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

// AuthMiddleware wraps a handler with authentication checks.
func AuthMiddleware(config *ConfigStore, sessions *SessionStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No auth if password not configured
		if !config.AuthEnabled() {
			next.ServeHTTP(w, r)
			return
		}

		// Bypass auth for localhost if configured
		if config.BypassAuthForLocalhost() && isLocalRequest(r) {
			next.ServeHTTP(w, r)
			return
		}

		// Allow login page and login POST without auth
		if r.URL.Path == "/login" || r.URL.Path == "/login.html" {
			next.ServeHTTP(w, r)
			return
		}

		// Check session cookie
		cookie, err := r.Cookie(sessionCookieName)
		if err == nil && sessions.Valid(cookie.Value) {
			next.ServeHTTP(w, r)
			return
		}

		// Not authenticated
		// API/WebSocket requests get 401
		if r.URL.Path == "/ws" || len(r.URL.Path) >= 4 && r.URL.Path[:4] == "/api" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Browser requests redirect to login
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
	})
}
