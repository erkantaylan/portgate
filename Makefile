.PHONY: build build-windows build-all docker-build docker-up docker-down clean

build:
	go build -o portgate .

build-windows:
	GOOS=windows GOARCH=amd64 go build -o portgate.exe .

build-all: build build-windows

docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

clean:
	rm -f portgate portgate.exe
