SHELL := /bin/bash

VERSION := v0.0.0

build:
	GOFLAGS="-ldflags=-X=main.build=$(VERSION)" \
	ko build -L -P -t $(VERSION) \
		--image-label org.opencontainers.image.version=$(VERSION) \
		--image-label org.opencontainers.image.source=$(shell git remote get-url origin) \
		--image-label org.opencontainers.image.revision=$(shell git rev-parse HEAD) \
		./cmd/rinha

build-prod:
	GOFLAGS="-ldflags=-X=main.build=$(VERSION)" \
	ko build -t $(VERSION) --base-import-paths \
		--image-label org.opencontainers.image.version=$(VERSION) \
		--image-label org.opencontainers.image.source=$(shell git remote get-url origin) \
		--image-label org.opencontainers.image.revision=$(shell git rev-parse HEAD) \
		./cmd/rinha

up:
	docker compose -f zarf/docker-compose.dev.yml up -d

down:
	docker compose -f zarf/docker-compose.dev.yml down

up-prod:
	docker compose -f zarf/docker-compose.yml up -d

down-prod:
	docker compose -f zarf/docker-compose.yml down

test:
	go test -count=1 ./...

testv:
	go test -race -v -count=1 ./...
