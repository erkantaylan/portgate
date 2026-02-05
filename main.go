package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		os.Args = append(os.Args, "start")
	}

	switch os.Args[1] {
	case "start":
		cmdStart()
	case "add":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "usage: portgate add <domain> <port>")
			os.Exit(1)
		}
		cmdAdd(os.Args[2], os.Args[3])
	case "remove":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: portgate remove <domain>")
			os.Exit(1)
		}
		cmdRemove(os.Args[2])
	case "list":
		cmdList()
	case "status":
		cmdStatus()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\ncommands: start, add, remove, list, status\n", os.Args[1])
		os.Exit(1)
	}
}

func cmdStart() {
	cs, err := NewConfigStore("")
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	hub := NewHub(cs)
	go hub.Run()

	scanner := NewScanner(10*time.Second, func(ports []DiscoveredPort) {
		hub.SetPorts(ports)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go scanner.Run(ctx)

	// Dashboard on :8080
	dashboardHandler := DashboardHandler(hub)
	dashSrv := &http.Server{Addr: ":8080", Handler: dashboardHandler}

	// Reverse proxy on :80
	proxyHandler := ProxyHandler(hub, "127.0.0.1:8080")
	proxySrv := &http.Server{Addr: ":80", Handler: proxyHandler}

	go func() {
		log.Printf("Dashboard listening on :8080")
		if err := dashSrv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("dashboard: %v", err)
		}
	}()

	go func() {
		log.Printf("Proxy listening on :80")
		if err := proxySrv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("proxy: %v", err)
		}
	}()

	log.Println("Portgate started")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Println("Shutting down...")
	cancel()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutCancel()
	dashSrv.Shutdown(shutCtx)
	proxySrv.Shutdown(shutCtx)
}

func cmdAdd(domain, portStr string) {
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		fmt.Fprintf(os.Stderr, "invalid port: %s\n", portStr)
		os.Exit(1)
	}
	body := fmt.Sprintf(`{"domain":"%s","port":%d}`, domain, port)
	resp, err := http.Post("http://localhost:8080/api/mappings", "application/json",
		strings.NewReader(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v (is portgate running?)\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated {
		fmt.Printf("Mapped %s.localhost → :%d\n", domain, port)
	} else {
		io.Copy(os.Stderr, resp.Body)
		os.Exit(1)
	}
}

func cmdRemove(domain string) {
	req, _ := http.NewRequest(http.MethodDelete,
		"http://localhost:8080/api/mappings?domain="+url.QueryEscape(domain), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v (is portgate running?)\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		fmt.Printf("Removed mapping for %s\n", domain)
	} else {
		io.Copy(os.Stderr, resp.Body)
		os.Exit(1)
	}
}

func cmdList() {
	resp, err := http.Get("http://localhost:8080/api/mappings")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v (is portgate running?)\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	var mappings []DomainMapping
	json.NewDecoder(resp.Body).Decode(&mappings)
	if len(mappings) == 0 {
		fmt.Println("No mappings configured")
		return
	}
	for _, m := range mappings {
		fmt.Printf("  %s.localhost → :%d\n", m.Domain, m.TargetPort)
	}
}

func cmdStatus() {
	resp, err := http.Get("http://localhost:8080/api/ports")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Portgate is not running\n")
		os.Exit(1)
	}
	defer resp.Body.Close()
	var ports []DiscoveredPort
	json.NewDecoder(resp.Body).Decode(&ports)
	fmt.Printf("Portgate is running — %d ports discovered\n", len(ports))
	for _, p := range ports {
		status := "●"
		if !p.Healthy {
			status = "○"
		}
		detail := p.ServiceName
		if p.Title != "" {
			detail += " — " + p.Title
		}
		fmt.Printf("  %s :%d  %s\n", status, p.Port, detail)
	}
}
