.PHONY: help proto build run clean test deps sqlc docker-up docker-down docker-logs db-migrate db-seed

# Forzar sh como shell (en Windows lo provee Git for Windows / scoop).
# Sin esto, make en Windows puede caer en cmd.exe y romper las recetas.
SHELL := sh

MODULE := github.com/can-you-buy-me
PROTOC_FLAGS := --proto_path=proto/v1 --go_out=. --go_opt=module=$(MODULE) --go-grpc_out=. --go-grpc_opt=module=$(MODULE)

help:
	@echo Can You Buy Me - Makefile Commands
	@echo   make proto       - Generate Go code from .proto files
	@echo   make sqlc        - Generate Go code from SQL queries
	@echo   make build       - Build the server binary
	@echo   make run         - Run the server locally
	@echo   make test        - Run tests
	@echo   make clean       - Clean build artifacts
	@echo   make docker-up   - Start Docker containers PostgreSQL Redis NATS
	@echo   make docker-down - Stop Docker containers

# Generar codigo Go desde archivos .proto
# protoc con --go_opt=module crea pkg/gen/<svc>/v1 automaticamente
proto:
	@echo Generating protobuf code...
	protoc $(PROTOC_FLAGS) health.proto
	protoc $(PROTOC_FLAGS) auth.proto
	protoc $(PROTOC_FLAGS) auction.proto
	protoc $(PROTOC_FLAGS) payment.proto
	@echo Protobuf code generated

# Generar codigo Go desde queries SQL (sqlc)
sqlc:
	@echo Generating sqlc code...
	sqlc generate
	@echo SQLC code generated

# Descargar dependencias
deps:
	@echo Downloading dependencies...
	go mod download
	go mod tidy
	@echo Dependencies ready

# Compilar el servidor
build: proto sqlc
	@echo Building server...
	go build -o bin/server ./cmd/server/main.go
	@echo Server built to bin/server

# Ejecutar el servidor
run: proto
	@echo Running server...
	go run ./cmd/server/main.go

# Ejecutar tests
test:
	@echo Running tests...
	go test -v ./...

# Limpiar
clean:
	@echo Cleaning...
	rm -rf bin
	rm -rf pkg/gen
	go clean
	@echo Cleaned

# Docker
docker-up:
	@echo Starting Docker containers...
	docker-compose -f api/containers/docker-compose.yml up -d
	@echo Containers started
	@echo   PostgreSQL: localhost:5435 user root password root db auction_db
	@echo   Redis:      localhost:6379
	@echo   NATS:       localhost:4222 monitoring http://localhost:8222

docker-down:
	@echo Stopping Docker containers...
	docker-compose -f api/containers/docker-compose.yml down
	@echo Containers stopped

docker-logs:
	docker-compose -f api/containers/docker-compose.yml logs -f

# Aplicar migraciones de base de datos
db-migrate:
	@echo Applying database migrations...
	psql postgresql://root:root@localhost:5435/auction_db -f api/sql/001_init.sql
	@echo Schema created

db-seed:
	@echo Seeding sample data...
	psql postgresql://root:root@localhost:5435/auction_db -f api/sql/002_seed_data.sql
	@echo Sample data loaded
