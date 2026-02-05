.PHONY: help build build-windows build-all run run-linux run-windows docker-build docker-up docker-down clean

BINARY      := portgate
BINARY_WIN  := portgate.exe

help: ## Show available targets
	@echo "Available targets:"
	@echo "  help             Show available targets"
	@echo "  build            Build for Linux"
	@echo "  build-windows    Cross-compile for Windows amd64"
	@echo "  build-all        Build for both Linux and Windows"
	@echo "  run              Build and run on Linux"
	@echo "  run-linux        Alias for run"
	@echo "  run-windows      Cross-compile Windows executable"
	@echo "  docker-build     Build Docker image"
	@echo "  docker-up        Start containers in background"
	@echo "  docker-down      Stop containers"
	@echo "  clean            Remove built binaries"

build: ## Build for Linux
	go build -o $(BINARY) .

build-windows: export GOOS := windows
build-windows: export GOARCH := amd64
build-windows: ## Cross-compile for Windows (amd64)
	go build -o $(BINARY_WIN) .

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

ifeq ($(OS),Windows_NT)
clean: ## Remove built binaries
	-del /f /q $(BINARY) $(BINARY_WIN) 2>nul
else
clean: ## Remove built binaries
	rm -f $(BINARY) $(BINARY_WIN)
endif
