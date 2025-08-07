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
