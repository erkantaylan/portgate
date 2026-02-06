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

// ProxyHandler returns an http.Handler that reverse-proxies based on Host header.
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

		// Reserved: bare localhost or portgate → dashboard
		if subdomain == "" || subdomain == "portgate" {
			proxyToDashboard(w, r, dashboardAddr)
			return
		}

		// Lookup mapping
		port := hub.config.LookupPort(subdomain)
		if port == 0 {
			// Unknown domain → redirect to dashboard
			http.Redirect(w, r, fmt.Sprintf("http://%s", dashboardAddr), http.StatusTemporaryRedirect)
			return
		}

		target := fmt.Sprintf("127.0.0.1:%d", port)

		// WebSocket upgrade detection
		if isWebSocketUpgrade(r) {
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
			},
			ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
				log.Printf("proxy error for %s: %v", subdomain, err)
				http.Error(w, "502 Bad Gateway", http.StatusBadGateway)
			},
		}
		proxy.ServeHTTP(w, r)
	})
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
