.PHONY: help proto build run clean test deps sqlc \
        db-status db-up db-down db-reset db-create db-seed install-goose \
        docker-up docker-down docker-logs

# Carga variables de .env (DB_URL, puertos, etc.) y las exporta a las recetas.
include .env
export

# Forzar sh como shell (en Windows lo provee Git for Windows / scoop).
# Sin esto, make en Windows puede caer en cmd.exe y romper las recetas.
SHELL := sh

MODULE := github.com/can-you-buy-me
# proto/ vive en la raíz del monorepo (compartido con el frontend Flutter).
# Este Makefile corre desde api/, por eso ../proto/v1.
PROTOC_FLAGS := --proto_path=../proto/v1 --go_out=. --go_opt=module=$(MODULE) --go-grpc_out=. --go-grpc_opt=module=$(MODULE)

help:
	@echo Can You Buy Me - Makefile Commands
	@echo   make proto       - Generate Go code from .proto files
	@echo   make sqlc        - Generate Go code from SQL queries
	@echo   make build       - Build the server binary
	@echo   make run         - Run the server locally
	@echo   make test        - Run tests
	@echo   make clean       - Clean build artifacts
	@echo   make db-status   - Show migration status (goose)
	@echo   make db-up       - Apply all pending migrations
	@echo   make db-down     - Rollback the last migration
	@echo   make db-reset    - Rollback ALL migrations (drops schema)
	@echo   make db-seed     - Load sample data (no-versioning)
	@echo   make db-create NAME=name - Create a new migration file
	@echo   make install-goose - Install goose (db migrations tool)
	@echo   make docker-up   - Start Docker containers PostgreSQL Redis NATS
	@echo   make docker-down - Stop Docker containers

# =====================================================
# Code generation
# =====================================================
# Generar codigo Go desde archivos .proto
# protoc con --go_opt=module crea pkg/gen/<svc>/v1 automaticamente
proto:
	@echo Generating protobuf code...
	protoc $(PROTOC_FLAGS) health.proto
	protoc $(PROTOC_FLAGS) auth.proto
	protoc $(PROTOC_FLAGS) auction.proto
	protoc $(PROTOC_FLAGS) payment.proto
	@echo Protobuf code generated

# Generar codigo Go desde queries SQL (sqlc lee el schema de db/migrations)
sqlc:
	@echo Generating sqlc code...
	sqlc generate
	@echo SQLC code generated

# =====================================================
# Build / run / test
# =====================================================
deps:
	@echo Downloading dependencies...
	go mod download
	go mod tidy
	@echo Dependencies ready

build: proto sqlc
	@echo Building server...
	go build -o bin/server ./cmd/server/main.go
	@echo Server built to bin/server

run: proto
	@echo Running server...
	go run ./cmd/server/main.go

test:
	@echo Running tests...
	go test -v ./...

clean:
	@echo Cleaning...
	rm -rf bin
	rm -rf pkg/gen
	go clean
	@echo Cleaned

# =====================================================
# Database migrations (Goose)
# =====================================================
db-status:
	@echo Checking migration status...
	goose -dir db/migrations postgres "$(DB_URL)" status

db-up:
	@echo Applying database migrations...
	goose -dir db/migrations postgres "$(DB_URL)" up

db-down:
	@echo Rolling back last migration...
	goose -dir db/migrations postgres "$(DB_URL)" down

db-reset:
	@echo Rolling back ALL migrations...
	goose -dir db/migrations postgres "$(DB_URL)" reset

db-create:
ifndef NAME
	$(error NAME is not set. Uso: make db-create NAME=nombre_migracion)
endif
	goose -dir db/migrations create $(NAME) sql

# Seed: datos de ejemplo. -no-versioning => no cuenta como migración de schema
# y es re-ejecutable (el seed hace TRUNCATE + INSERT).
db-seed:
	@echo Seeding sample data...
	goose -no-versioning -dir db/seeds postgres "$(DB_URL)" up
	@echo Sample data loaded

install-goose:
	@echo Installing goose...
	go install github.com/pressly/goose/v3/cmd/goose@latest
	@echo goose installed

# =====================================================
# Docker (local dev)
# =====================================================
docker-up:
	@echo Starting Docker containers...
	docker-compose -f containers/docker-compose.yml up -d
	@echo Containers started
	@echo   PostgreSQL: localhost:5435 user root password root db auction_db
	@echo   Redis:      localhost:6379
	@echo   NATS:       localhost:4222 monitoring http://localhost:8222

docker-down:
	@echo Stopping Docker containers...
	docker-compose -f containers/docker-compose.yml down
	@echo Containers stopped

docker-logs:
	docker-compose -f containers/docker-compose.yml logs -f
