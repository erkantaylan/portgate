.PHONY: build docker-build docker-up docker-down clean

build:
	go build -o portgate .

docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

clean:
	rm -f portgate
