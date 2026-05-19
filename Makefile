.PHONY: default help install build require-secrets backend server api backend\:debug server\:debug api\:debug backend\:debug-logs server\:debug-logs api\:debug-logs test lint proto-gen proto-gen-web frontend web web-dev frontend\:build web-build frontend\:lint web-lint dev dev\:debug check clean

BIN_DIR ?= bin
DATA_DIR ?= data
DEBUG_DIR ?= debug
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
GARDENCTL := $(BIN_DIR)/gardenctl$(EXE)

default: help

help:
	@echo "Available targets:"
	@echo "  install              Install gardend and gardenctl to GOPATH/bin"
	@echo "  build                Build binaries to $(BIN_DIR)/"
	@echo "  backend | server     Start gardend API server"
	@echo "  backend:debug        Start gardend with debug logs and JSONL output"
	@echo "  test                 Run go tests"
	@echo "  lint                 Run golangci-lint"
	@echo "  proto-gen            Generate Go protobuf code"
	@echo "  proto-gen-web        Generate TypeScript protobuf code"
	@echo "  frontend | web-dev   Start Next.js dev server"
	@echo "  frontend:build       Build Next.js for production"
	@echo "  frontend:lint        Run frontend lint"
	@echo "  dev                  Start backend and frontend together"
	@echo "  dev:debug            Start debug backend and frontend together"
	@echo "  check                Run backend tests plus frontend lint/build"
	@echo "  clean                Remove build artifacts"

install:
	go install ./cmd/gardend
	go install ./cmd/gardenctl

build:
	$(MKDIR_BIN)
	go build -o $(GARDEND) ./cmd/gardend
	go build -o $(GARDENCTL) ./cmd/gardenctl

require-secrets:
	@test -n "$(JWT_SECRET)" || (echo "JWT_SECRET is required" && exit 1)
	@test -n "$(ADMIN_PASSWORD)" || (echo "ADMIN_PASSWORD is required" && exit 1)

backend: require-secrets
	go run ./cmd/gardend serve --data-dir "$(DATA_DIR)" --listen "$(LISTEN)" --jwt-secret "$(JWT_SECRET)" --admin-username "$(ADMIN_USERNAME)" --admin-password "$(ADMIN_PASSWORD)" --admin-email "$(ADMIN_EMAIL)" --cors-origins "$(CORS_ORIGINS)" --log-level "$(SERVER_LOG_LEVEL)" --log-format "$(SERVER_LOG_FORMAT)"

server api: backend

backend\:debug: require-secrets
	go run ./cmd/gardend serve --data-dir "$(DATA_DIR)" --listen "$(LISTEN)" --jwt-secret "$(JWT_SECRET)" --admin-username "$(ADMIN_USERNAME)" --admin-password "$(ADMIN_PASSWORD)" --admin-email "$(ADMIN_EMAIL)" --cors-origins "$(CORS_ORIGINS)" --log-level debug --log-format "$(SERVER_LOG_FORMAT)" --debug-dir "$(DEBUG_DIR)"

server\:debug api\:debug: backend\:debug
backend\:debug-logs server\:debug-logs api\:debug-logs: backend\:debug

test:
	go test -count=1 ./...

lint:
	golangci-lint run ./...

proto-gen:
	buf generate

proto-gen-web:
	buf generate --template buf.gen.web.yaml

frontend:
	$(FRONTEND_DEV)

web web-dev: frontend

frontend\:build:
	$(FRONTEND_BUILD)

web-build: frontend\:build

frontend\:lint:
	pnpm --dir web lint

web-lint: frontend\:lint

dev:
	$(MAKE) -j2 backend frontend

dev\:debug:
	$(MAKE) -j2 backend:debug frontend

check: test frontend\:lint frontend\:build

clean:
	$(RM_BIN)
