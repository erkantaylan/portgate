package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

// ProxyHandler returns an http.Handler that reverse-proxies based on Host header
// (subdomain routing) and URL path (path-based routing for external access).
// Reserved subdomains: "portgate" → dashboard, bare "localhost" → dashboard.
func ProxyHandler(hub *Hub, dashboardAddr string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		// Strip port if present
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}

		suffix := hub.config.DomainSuffix()
		subdomain := extractSubdomain(host, suffix)

		// If subdomain routing matched, use it
		if subdomain != "" && subdomain != "portgate" {
			port := hub.config.LookupPort(subdomain)
			if port != 0 {
				proxyToPort(w, r, subdomain, port, "")
				return
			}
		}

		// Try path-based routing: /{domain-name}/rest/of/path
		if pathDomain, remaining := extractPathDomain(r.URL.Path); pathDomain != "" {
			port := hub.config.LookupPort(pathDomain)
			if port != 0 {
				proxyToPort(w, r, pathDomain, port, remaining)
				return
			}
		}

		// Everything else → dashboard
		proxyToDashboard(w, r, dashboardAddr)
	})
}

// extractPathDomain extracts the first path segment as a potential domain name.
// Returns the domain and the remaining path (with leading /).
// e.g. "/myapp/api/data" → ("myapp", "/api/data")
// e.g. "/" → ("", "")
func extractPathDomain(path string) (string, string) {
	// Remove leading slash
	trimmed := strings.TrimPrefix(path, "/")
	if trimmed == "" {
		return "", ""
	}
	// Split on first slash
	parts := strings.SplitN(trimmed, "/", 2)
	domain := parts[0]
	remaining := "/"
	if len(parts) > 1 {
		remaining = "/" + parts[1]
	}
	return domain, remaining
}

// proxyToPort reverse-proxies to the given port, optionally rewriting the path.
// If pathPrefix is non-empty, the request URL path is set to that value
// (stripping the domain-name prefix used in path-based routing).
func proxyToPort(w http.ResponseWriter, r *http.Request, name string, port int, rewritePath string) {
	target := fmt.Sprintf("127.0.0.1:%d", port)

	// WebSocket upgrade detection
	if isWebSocketUpgrade(r) {
		if rewritePath != "" {
			r.URL.Path = rewritePath
		}
		handleWebSocket(w, r, target)
		return
	}

	// Regular HTTP reverse proxy
	proxyURL, _ := url.Parse(fmt.Sprintf("http://%s", target))
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = proxyURL.Scheme
			req.URL.Host = proxyURL.Host
			req.Host = r.Host
			if rewritePath != "" {
				req.URL.Path = rewritePath
				// Preserve query string
				req.URL.RawQuery = r.URL.RawQuery
			}
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("proxy error for %s: %v", name, err)
			http.Error(w, "502 Bad Gateway", http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(w, r)
}

func extractSubdomain(host, suffix string) string {
	// host is like "livemd.localhost" or "localhost"
	dotSuffix := "." + suffix
	if !strings.HasSuffix(host, dotSuffix) {
		return ""
	}
	sub := strings.TrimSuffix(host, dotSuffix)
	if sub == "" {
		return ""
	}
	return sub
}

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Connection"), "upgrade") &&
		strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

func handleWebSocket(w http.ResponseWriter, r *http.Request, target string) {
	// Dial backend
	backendConn, err := net.DialTimeout("tcp", target, 5*time.Second)
	if err != nil {
		http.Error(w, "502 Bad Gateway", http.StatusBadGateway)
		return
	}

	// Hijack client connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "websocket hijack failed", http.StatusInternalServerError)
		backendConn.Close()
		return
	}
	clientConn, clientBuf, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, "websocket hijack failed", http.StatusInternalServerError)
		backendConn.Close()
		return
	}

	// Forward the original request to backend
	if err := r.Write(backendConn); err != nil {
		clientConn.Close()
		backendConn.Close()
		return
	}

	// Flush any buffered data from client
	if clientBuf.Reader.Buffered() > 0 {
		buffered := make([]byte, clientBuf.Reader.Buffered())
		clientBuf.Read(buffered)
		backendConn.Write(buffered)
	}

	// Bidirectional copy
	go func() {
		io.Copy(backendConn, clientConn)
		backendConn.Close()
	}()
	go func() {
		io.Copy(clientConn, backendConn)
		clientConn.Close()
	}()
}

func proxyToDashboard(w http.ResponseWriter, r *http.Request, dashboardAddr string) {
	proxyURL, _ := url.Parse(fmt.Sprintf("http://%s", dashboardAddr))
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = proxyURL.Scheme
			req.URL.Host = proxyURL.Host
		},
	}
	proxy.ServeHTTP(w, r)
}
