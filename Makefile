.PHONY: help build build-windows build-all run run-linux run-windows docker-build docker-up docker-down clean install-service uninstall-service status-service release

BINARY      := portgate
BINARY_WIN  := portgate.exe
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -X main.version=$(VERSION)

help: ## Show available targets
	@echo "Available targets:"
	@echo "  help               Show available targets"
	@echo "  build              Build for Linux (VERSION=$(VERSION))"
	@echo "  build-windows      Cross-compile for Windows amd64"
	@echo "  build-all          Build for both Linux and Windows"
	@echo "  release            Build release binaries for all platforms"
	@echo "  run                Build and run on Linux"
	@echo "  run-linux          Alias for run"
	@echo "  run-windows        Cross-compile Windows executable"
	@echo "  docker-build       Build Docker image"
	@echo "  docker-up          Start containers in background"
	@echo "  docker-down        Stop containers"
	@echo "  clean              Remove built binaries"
	@echo "  install-service    Install portgate as OS startup service"
	@echo "  uninstall-service  Remove portgate OS startup service"
	@echo "  status-service     Check portgate service status"

build: ## Build for Linux
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

build-windows: export GOOS := windows
build-windows: export GOARCH := amd64
build-windows: ## Cross-compile for Windows (amd64)
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_WIN) .

build-all: build build-windows ## Build for both Linux and Windows

run: build ## Build and run on Linux
	./$(BINARY)

run-linux: run ## Alias for run

run-windows: build-windows ## Cross-compile Windows executable
	@echo "Built $(BINARY_WIN) — copy to a Windows host to run"

docker-build: ## Build Docker image
	docker compose build

docker-up: ## Start containers in background
	docker compose up -d

docker-down: ## Stop containers
	docker compose down

release: ## Build release binaries for all platforms (VERSION=v1.0.0)
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o portgate-linux-amd64 .
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o portgate-windows-amd64.exe .

ifeq ($(OS),Windows_NT)

INSTALL_DIR    := $(LOCALAPPDATA)\portgate
STARTUP_DIR    := $(shell powershell -NoProfile -Command "[Environment]::GetFolderPath('Startup')")
STARTUP_SCRIPT := $(STARTUP_DIR)\portgate.vbs

install-service: build-windows ## Install portgate as Windows startup service
	@if not exist "$(INSTALL_DIR)" mkdir "$(INSTALL_DIR)"
	copy /y $(BINARY_WIN) "$(INSTALL_DIR)\$(BINARY_WIN)"
	@echo Set WshShell = CreateObject("WScript.Shell") > "$(STARTUP_SCRIPT)"
	@echo WshShell.Run """$(INSTALL_DIR)\$(BINARY_WIN)"" start", 0 >> "$(STARTUP_SCRIPT)"
	@echo Portgate installed to $(INSTALL_DIR) and registered for startup

uninstall-service: ## Remove portgate Windows startup service
	-taskkill /f /im $(BINARY_WIN) 2>nul
	-del /f /q "$(STARTUP_SCRIPT)" 2>nul
	-rmdir /s /q "$(INSTALL_DIR)" 2>nul
	@echo Portgate startup service removed

status-service: ## Check portgate service status (Windows)
	@tasklist /fi "imagename eq $(BINARY_WIN)" 2>nul | find /i "$(BINARY_WIN)" >nul && (echo Portgate is running) || (echo Portgate is not running)

clean: ## Remove built binaries
	-del /f /q $(BINARY) $(BINARY_WIN) 2>nul

else

INSTALL_DIR  := /usr/local/bin
SERVICE_FILE := /etc/systemd/system/portgate.service

define SYSTEMD_UNIT
[Unit]
Description=Portgate - Local port discovery and reverse proxy
After=network.target

[Service]
Type=simple
ExecStart=$(INSTALL_DIR)/$(BINARY) start
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
endef
export SYSTEMD_UNIT

install-service: build ## Install portgate as systemd service (requires sudo)
	install -m 755 $(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "$$SYSTEMD_UNIT" > $(SERVICE_FILE)
	systemctl daemon-reload
	systemctl enable portgate
	systemctl start portgate
	@echo "Portgate service installed and started"

uninstall-service: ## Remove portgate systemd service (requires sudo)
	-systemctl stop portgate
	-systemctl disable portgate
	rm -f $(SERVICE_FILE)
	systemctl daemon-reload
	rm -f $(INSTALL_DIR)/$(BINARY)
	@echo "Portgate service removed"

status-service: ## Check portgate service status
	@systemctl is-active portgate >/dev/null 2>&1 && echo "Portgate is running" || echo "Portgate is not running"
	-@systemctl status portgate --no-pager 2>/dev/null

clean: ## Remove built binaries
	rm -f $(BINARY) $(BINARY_WIN)

endif
