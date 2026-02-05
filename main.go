package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"
)

var version = "dev"

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
	case "scan-range":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: portgate scan-range <add|remove|list> [start-end]")
			os.Exit(1)
		}
		cmdScanRange(os.Args[2:])
	case "add-port":
		cmdAddPort(os.Args[2:])
	case "remove-port":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: portgate remove-port <port>")
			os.Exit(1)
		}
		cmdRemovePort(os.Args[2])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\ncommands: start, add, remove, list, status, scan-range, add-port, remove-port\n", os.Args[1])
		os.Exit(1)
	}
}

func cmdStart() {
	startFlags := flag.NewFlagSet("start", flag.ExitOnError)
	dashPort := startFlags.Int("dashboard-port", 8080, "dashboard listen port")
	proxyPort := startFlags.Int("proxy-port", 80, "reverse proxy listen port")
	startFlags.Parse(os.Args[2:])

	cs, err := NewConfigStore("")
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	hub := NewHub(cs)
	go hub.Run()

	scanner := NewScanner(10*time.Second, cs, func(ports []DiscoveredPort) {
		hub.SetPorts(ports)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go scanner.Run(ctx)

	dashAddr := fmt.Sprintf(":%d", *dashPort)
	proxyAddr := fmt.Sprintf(":%d", *proxyPort)

	// Dashboard
	dashboardHandler := DashboardHandler(hub)
	dashSrv := &http.Server{Addr: dashAddr, Handler: dashboardHandler}

	// Reverse proxy
	proxyHandler := ProxyHandler(hub, fmt.Sprintf("127.0.0.1:%d", *dashPort))
	proxySrv := &http.Server{Addr: proxyAddr, Handler: proxyHandler}

	go func() {
		log.Printf("Dashboard listening on %s", dashAddr)
		if err := dashSrv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("dashboard: %v", err)
		}
	}()

	go func() {
		log.Printf("Proxy listening on %s", proxyAddr)
		if err := proxySrv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("proxy: %v", err)
		}
	}()

	log.Println("Portgate started")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, shutdownSignals...)
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
		source := ""
		if p.Source == "manual" {
			source = " [manual]"
		}
		detail := p.ServiceName
		if p.Title != "" {
			detail += " — " + p.Title
		}
		fmt.Printf("  %s :%d  %s%s\n", status, p.Port, detail, source)
	}
}

func cmdScanRange(args []string) {
	switch args[0] {
	case "list":
		cs, err := NewConfigStore("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "config: %v\n", err)
			os.Exit(1)
		}
		ranges := cs.ScanRanges()
		if len(ranges) == 0 {
			fmt.Println("No scan ranges configured")
			return
		}
		fmt.Println("Scan ranges:")
		for _, r := range ranges {
			fmt.Printf("  %d-%d\n", r.Start, r.End)
		}

	case "add":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: portgate scan-range add <start>-<end>")
			os.Exit(1)
		}
		sr := parseScanRange(args[1])
		cs, err := NewConfigStore("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "config: %v\n", err)
			os.Exit(1)
		}
		if err := cs.AddScanRange(sr); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Added scan range %d-%d\n", sr.Start, sr.End)

	case "remove":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: portgate scan-range remove <start>-<end>")
			os.Exit(1)
		}
		sr := parseScanRange(args[1])
		cs, err := NewConfigStore("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "config: %v\n", err)
			os.Exit(1)
		}
		if err := cs.RemoveScanRange(sr); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Removed scan range %d-%d\n", sr.Start, sr.End)

	default:
		fmt.Fprintf(os.Stderr, "unknown scan-range subcommand: %s\nsubcommands: add, remove, list\n", args[0])
		os.Exit(1)
	}
}

func parseScanRange(s string) ScanRange {
	var start, end int
	n, err := fmt.Sscanf(s, "%d-%d", &start, &end)
	if err != nil || n != 2 || start > end || start < 1 || end > 65535 {
		fmt.Fprintf(os.Stderr, "invalid range: %s (expected start-end, e.g. 9000-9999)\n", s)
		os.Exit(1)
	}
	return ScanRange{Start: start, End: end}
}

func cmdAddPort(args []string) {
	fs := flag.NewFlagSet("add-port", flag.ExitOnError)
	name := fs.String("name", "", "optional name for the port")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: portgate add-port <port> [--name \"my-app\"]")
		os.Exit(1)
	}

	var port int
	if _, err := fmt.Sscanf(fs.Arg(0), "%d", &port); err != nil || port < 1 || port > 65535 {
		fmt.Fprintf(os.Stderr, "invalid port: %s\n", fs.Arg(0))
		os.Exit(1)
	}

	cs, err := NewConfigStore("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	mp := ManualPort{Port: port, Name: *name}
	if err := cs.AddManualPort(mp); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if *name != "" {
		fmt.Printf("Registered port %d (%s)\n", port, *name)
	} else {
		fmt.Printf("Registered port %d\n", port)
	}
}

func cmdRemovePort(portStr string) {
	var port int
	if _, err := fmt.Sscanf(portStr, "%d", &port); err != nil {
		fmt.Fprintf(os.Stderr, "invalid port: %s\n", portStr)
		os.Exit(1)
	}
	cs, err := NewConfigStore("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	if err := cs.RemoveManualPort(port); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Removed manual port %d\n", port)
}
