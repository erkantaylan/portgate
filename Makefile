.PHONY: help build build-windows build-all run run-linux run-windows docker-build docker-up docker-down clean

BINARY      := portgate
BINARY_WIN  := portgate.exe

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

build: ## Build for Linux
	go build -o $(BINARY) .

build-windows: ## Cross-compile for Windows (amd64)
	GOOS=windows GOARCH=amd64 go build -o $(BINARY_WIN) .

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

clean: ## Remove built binaries
	rm -f $(BINARY) $(BINARY_WIN)
