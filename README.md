# Portgate

Portgate is a local reverse proxy and port discovery dashboard. It automatically scans for services running on your machine, displays them in a live-updating web dashboard, and lets you route `*.localhost` subdomains to any local port.

## Features

- **Port scanning** — automatically discovers services on configurable port ranges
- **Subdomain routing** — map `myapp.localhost` to any local port via `*.localhost` convention
- **Manual port registration** — register ports that fall outside scan ranges
- **Health checking** — continuously monitors whether discovered services are up
- **WebSocket live updates** — dashboard refreshes in real time as ports come and go
- **HTTP service detection** — probes discovered ports for HTTP, extracts page titles and server headers
- **Cross-platform** — runs on Linux and Windows

## Quick Start

```bash
# Build and run
make run

# Or build and run manually
go build -o portgate .
./portgate start
```

Open [http://localhost:8080](http://localhost:8080) to see the dashboard, or [http://portgate.localhost](http://portgate.localhost) through the proxy on port 80.

## Installation

### Prerequisites

- Go 1.22+
- make (optional)

### Build from Source

**Linux:**

```bash
git clone <repo-url>
cd portgate
make build        # produces ./portgate
```

**Windows (cross-compile from Linux):**

```bash
make build-windows  # produces ./portgate.exe
```

**Both platforms:**

```bash
make build-all    # produces ./portgate and ./portgate.exe
```

## Usage

If no command is given, `portgate` defaults to `start`.

### `portgate start`

Start the dashboard and reverse proxy.

```bash
portgate start [--dashboard-port 8080] [--proxy-port 80]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--dashboard-port` | `8080` | Port for the web dashboard and API |
| `--proxy-port` | `80` | Port for the subdomain reverse proxy |

### `portgate add <domain> <port>`

Create a subdomain mapping. Routes `<domain>.localhost` to the given port.

```bash
portgate add myapp 3000
# Mapped myapp.localhost → :3000
```

### `portgate remove <domain>`

Remove a subdomain mapping.

```bash
portgate remove myapp
# Removed mapping for myapp
```

### `portgate list`

List all configured subdomain mappings.

```bash
portgate list
#   myapp.localhost → :3000
#   api.localhost → :4000
```

### `portgate status`

Show whether Portgate is running and list discovered ports with health status.

```bash
portgate status
# Portgate is running — 3 ports discovered
#   ● :3000  http — My App
#   ● :8080  http — Portgate
#   ○ :9090  tcp [manual]
```

`●` = healthy, `○` = unreachable. `[manual]` indicates a manually registered port.

### `portgate add-port <port> [--name <name>]`

Register a port manually. Useful for services outside the default scan ranges.

```bash
portgate add-port 9090 --name "prometheus"
# Registered port 9090 (prometheus)
```

### `portgate remove-port <port>`

Remove a manually registered port.

```bash
portgate remove-port 9090
# Removed manual port 9090
```

### `portgate scan-range <add|remove|list>`

Manage port scan ranges.

```bash
# List current ranges
portgate scan-range list

# Add a range
portgate scan-range add 9000-9999

# Remove a range
portgate scan-range remove 3000-3999
```

## Configuration

Configuration is stored as JSON and created automatically on first run.

| Platform | Path |
|----------|------|
| Linux | `~/.config/portgate/config.json` |
| Windows | `%APPDATA%\portgate\config.json` |

### Config Fields

```json
{
  "mappings": [
    { "domain": "myapp", "targetPort": 3000, "createdAt": "..." }
  ],
  "scanIntervalSec": 10,
  "scanRanges": [
    { "start": 3000, "end": 3999 },
    { "start": 4000, "end": 4099 },
    { "start": 5000, "end": 5999 },
    { "start": 8000, "end": 8999 }
  ],
  "manualPorts": [
    { "port": 9090, "name": "prometheus" }
  ]
}
```

| Field | Description |
|-------|-------------|
| `mappings` | Subdomain-to-port routing rules |
| `scanIntervalSec` | Seconds between scan cycles (default: 10) |
| `scanRanges` | Port ranges to scan (defaults shown above) |
| `manualPorts` | Manually registered ports with optional names |

## How It Works

**Subdomain routing:** Portgate listens on the proxy port (default 80) and inspects the `Host` header. A request to `myapp.localhost` extracts `myapp` as the subdomain, looks up the mapping, and reverse-proxies to the target port. Bare `localhost` and `portgate.localhost` route to the dashboard.

**Port scanning:** A background scanner runs on a configurable interval (default 10s). It attempts TCP connections to every port in the configured scan ranges. For open ports, it probes for HTTP and extracts `<title>` tags and `Server` headers to identify services.

**WebSocket updates:** The dashboard connects via WebSocket at `/ws`. When the scanner completes a cycle, updated port and mapping data is broadcast to all connected clients in real time.

**Reverse proxy:** Both regular HTTP and WebSocket connections are proxied. WebSocket upgrades are detected and handled via TCP connection hijacking for bidirectional forwarding.

## API

All endpoints are served on the dashboard port (default 8080).

### Mappings

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/mappings` | List all domain mappings |
| `POST` | `/api/mappings` | Create a mapping (`{"domain": "myapp", "port": 3000}`) |
| `DELETE` | `/api/mappings?domain=myapp` | Remove a mapping |

### Ports

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/ports` | List all discovered ports |
| `POST` | `/api/ports` | Register a manual port (`{"port": 9090, "name": "my-svc"}`) |
| `DELETE` | `/api/ports?port=9090` | Remove a manual port |

### Scan Ranges

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/scan-ranges` | List scan ranges |
| `POST` | `/api/scan-ranges` | Add a range (`{"start": 9000, "end": 9999}`) |
| `DELETE` | `/api/scan-ranges?start=9000&end=9999` | Remove a range |

### WebSocket

| Endpoint | Description |
|----------|-------------|
| `/ws` | Real-time updates (ports and mappings) |

Messages are JSON with the format `{"type": "update", "data": {"ports": [...], "mappings": [...]}}`.

## Docker

**Linux** (uses host networking for port scanning):

```bash
docker compose up -d
```

**Windows** (uses port mapping since host networking is unavailable):

```bash
docker compose -f docker-compose.windows.yml up -d
```

Stop:

```bash
docker compose down
```

## Makefile Targets

| Target | Description |
|--------|-------------|
| `help` | Show available targets |
| `build` | Build for Linux |
| `build-windows` | Cross-compile for Windows (amd64) |
| `build-all` | Build for both Linux and Windows |
| `run` | Build and run on Linux |
| `run-linux` | Alias for `run` |
| `run-windows` | Cross-compile Windows executable |
| `docker-build` | Build Docker image |
| `docker-up` | Start containers in background |
| `docker-down` | Stop containers |
| `clean` | Remove built binaries |
