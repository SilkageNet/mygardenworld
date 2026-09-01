.PHONY: default help install build reset-data compact-db catalog-gen require-secrets backend server api test test-race vet lint proto-gen proto-gen-web proto-check web-deps frontend web web-dev web-build web-lint web-test dev check clean

BIN_DIR ?= bin
DATA_DIR ?= data
DEBUG_DIR ?= debug
MINI_DIR ?= tmp/mini
CATALOG_CDN ?=
LISTEN ?= 127.0.0.1:50051
FRONTEND_HOST ?= 127.0.0.1
FRONTEND_PORT ?= 3000
API_URL ?= http://127.0.0.1:50051
JWT_SECRET ?=
ADMIN_USERNAME ?= admin
ADMIN_PASSWORD ?=
ADMIN_EMAIL ?= admin@localhost
CORS_ORIGINS ?= http://localhost:3000,http://127.0.0.1:3000
SERVER_LOG_LEVEL ?= info
SERVER_LOG_FORMAT ?= text
LOG_RETENTION_DAYS ?= 7

ifeq ($(OS),Windows_NT)
	EXE := .exe
	MKDIR_BIN := powershell -NoProfile -Command "New-Item -ItemType Directory -Force -Path '$(BIN_DIR)' | Out-Null"
	RM_BIN := powershell -NoProfile -Command "if (Test-Path '$(BIN_DIR)') { Remove-Item -Recurse -Force '$(BIN_DIR)' }"
	FRONTEND_DEV := set NEXT_PUBLIC_API_URL=$(API_URL)&& pnpm --dir web dev --hostname $(FRONTEND_HOST) --port $(FRONTEND_PORT)
	FRONTEND_BUILD := set NEXT_PUBLIC_API_URL=$(API_URL)&& pnpm --dir web build
else
	EXE :=
	MKDIR_BIN := mkdir -p $(BIN_DIR)
	RM_BIN := rm -rf $(BIN_DIR)
	FRONTEND_DEV := NEXT_PUBLIC_API_URL="$(API_URL)" pnpm --dir web dev --hostname "$(FRONTEND_HOST)" --port "$(FRONTEND_PORT)"
	FRONTEND_BUILD := NEXT_PUBLIC_API_URL="$(API_URL)" pnpm --dir web build
endif

GARDEND := $(BIN_DIR)/gardend$(EXE)
GOLANGCI_LINT ?= go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.0

default: help

help:
	@echo "Available targets:"
	@echo "  install              Install gardend to GOPATH/bin"
	@echo "  build                Build gardend to $(BIN_DIR)/"
	@echo "  gardencap            Capture proxy source is available under cmd/gardencap"
	@echo "  catalog-gen          Refresh client-derived protocol and catalog artifacts from MINI_DIR"
	@echo "  reset-data           Delete local DATA_DIR via gardend reset-data"
	@echo "  compact-db           Reclaim unused SQLite space after stopping gardend"
	@echo "  backend | server     Start gardend API server"
	@echo "  backend:debug        Start gardend with debug logs and JSONL output"
	@echo "  test                 Run go tests"
	@echo "  test-race            Run go tests with the race detector"
	@echo "  vet                  Run go vet"
	@echo "  lint                 Run golangci-lint"
	@echo "  proto-gen            Generate Go protobuf code"
	@echo "  proto-gen-web        Generate TypeScript protobuf code"
	@echo "  proto-check          Verify protobuf generation is current"
	@echo "  frontend:deps        Install frontend dependencies"
	@echo "  frontend | web-dev   Start Next.js dev server"
	@echo "  frontend:build       Build Next.js for production"
	@echo "  frontend:lint        Run frontend lint"
	@echo "  frontend:test        Run frontend unit tests"
	@echo "  dev                  Start backend and frontend together"
	@echo "  dev:debug            Start debug backend and frontend together"
	@echo "  check                Run backend tests plus frontend lint/build"
	@echo "  clean                Remove build artifacts"

install:
	go install ./cmd/gardend

build:
	$(MKDIR_BIN)
	go build -o $(GARDEND) ./cmd/gardend

catalog-gen:
	go run ./cmd/gardencatalog --mini "$(MINI_DIR)" --cdn "$(CATALOG_CDN)" --state "internal/state/catalog_data.json" --web "web/src/lib/game/catalog.json" --protocol-package "internal/babigame/clientproto" --rpc-facade "internal/babigame/clientrpc/rpc_facade.go"

reset-data:
	go run ./cmd/gardend reset-data --data-dir "$(DATA_DIR)" --yes

compact-db:
	go run ./cmd/gardend compact-db --data-dir "$(DATA_DIR)" --yes

require-secrets:
	@test -n "$(JWT_SECRET)" || (echo "JWT_SECRET is required" && exit 1)
	@test -n "$(ADMIN_PASSWORD)" || (echo "ADMIN_PASSWORD is required" && exit 1)

backend: require-secrets
	go run ./cmd/gardend serve --data-dir "$(DATA_DIR)" --listen "$(LISTEN)" --jwt-secret "$(JWT_SECRET)" --admin-username "$(ADMIN_USERNAME)" --admin-password "$(ADMIN_PASSWORD)" --admin-email "$(ADMIN_EMAIL)" --cors-origins "$(CORS_ORIGINS)" --log-level "$(SERVER_LOG_LEVEL)" --log-format "$(SERVER_LOG_FORMAT)" --log-retention-days "$(LOG_RETENTION_DAYS)"

server api: backend

backend\:debug: require-secrets
	go run ./cmd/gardend serve --data-dir "$(DATA_DIR)" --listen "$(LISTEN)" --jwt-secret "$(JWT_SECRET)" --admin-username "$(ADMIN_USERNAME)" --admin-password "$(ADMIN_PASSWORD)" --admin-email "$(ADMIN_EMAIL)" --cors-origins "$(CORS_ORIGINS)" --log-level debug --log-format "$(SERVER_LOG_FORMAT)" --debug-dir "$(DEBUG_DIR)" --log-retention-days "$(LOG_RETENTION_DAYS)"

server\:debug api\:debug:
	$(MAKE) backend:debug

backend\:debug-logs server\:debug-logs api\:debug-logs:
	$(MAKE) backend:debug

test:
	go test -count=1 ./...

test-race:
	go test -race -count=1 ./...

vet:
	go vet ./...

lint:
	$(GOLANGCI_LINT) run

proto-gen:
	buf generate

proto-gen-web:
	buf generate --template buf.gen.web.yaml

proto-check: proto-gen proto-gen-web
	git diff --exit-code -- gen web/src/gen

frontend\:deps:
	pnpm --dir web install --frozen-lockfile

web-deps:
	$(MAKE) frontend:deps

frontend: web-deps
	$(FRONTEND_DEV)

web web-dev: frontend

frontend\:build: web-deps
	$(FRONTEND_BUILD)

web-build:
	$(MAKE) frontend:build

frontend\:lint: web-deps
	pnpm --dir web lint

web-lint:
	$(MAKE) frontend:lint

frontend\:test: web-deps
	pnpm --dir web test

web-test:
	$(MAKE) frontend:test

dev:
	$(MAKE) -j2 backend frontend

dev\:debug:
	$(MAKE) -j2 backend:debug frontend

check: test vet lint web-test web-lint web-build

clean:
	$(RM_BIN)
