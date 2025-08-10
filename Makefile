SHELL := /bin/bash

build:
	go mod download
	go build -o bin/spyder ./cmd/spyder

lint:
	golangci-lint run

test:
	go test ./... -coverprofile=coverage.txt

docker:
	docker build -t spyder-probe:latest .

run:
	./bin/spyder -domains=configs/domains.txt

docs:
	cd docs && npm i && npm run docs:dev

# Docker operations
docker-build:
	docker build -t spyder-probe:latest .

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f spyder

docker-dev:
	docker-compose -f docker-compose.dev.yml up

docker-dev-down:
	docker-compose -f docker-compose.dev.yml down

docker-clean:
	docker-compose down -v
	docker-compose -f docker-compose.dev.yml down -v
