.PHONY: help proto build run clean test

help:
	@echo "Can You Buy Me - Makefile Commands"
	@echo ""
	@echo "Commands:"
	@echo "  make proto       - Generate Go code from .proto files"
	@echo "  make build       - Build the server binary"
	@echo "  make run         - Run the server locally"
	@echo "  make test        - Run tests"
	@echo "  make clean       - Clean build artifacts"
	@echo "  make docker-up   - Start Docker containers (PostgreSQL, Redis, NATS)"
	@echo "  make docker-down - Stop Docker containers"

# Generar código Go desde archivos .proto
proto:
	@echo "Generating protobuf code..."
	@mkdir -p pkg/gen/health/v1 pkg/gen/auth/v1 pkg/gen/auction/v1 pkg/gen/payment/v1
	protoc --go_out=. --go-grpc_out=. proto/v1/health.proto
	protoc --go_out=. --go-grpc_out=. proto/v1/auth.proto
	protoc --go_out=. --go-grpc_out=. proto/v1/auction.proto
	protoc --go_out=. --go-grpc_out=. proto/v1/payment.proto
	@echo "✓ Protobuf code generated"

# Generar código Go desde queries SQL (sqlc)
sqlc:
	@echo "Generating sqlc code..."
	@command -v sqlc >/dev/null 2>&1 || (echo "sqlc not found. Install with: go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest" && exit 1)
	sqlc generate
	@echo "✓ SQLC code generated"

# Descargar dependencias
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "✓ Dependencies ready"

# Compilar el servidor
build: proto sqlc
	@echo "Building server..."
	go build -o bin/server ./cmd/server/main.go
	@echo "✓ Server built to bin/server"

# Ejecutar el servidor
run: proto
	@echo "Running server..."
	go run ./cmd/server/main.go

# Ejecutar tests
test:
	@echo "Running tests..."
	go test -v ./...

# Limpiar
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -rf pkg/gen/
	go clean
	@echo "✓ Cleaned"

# Docker
docker-up:
	@echo "Starting Docker containers..."
	docker-compose -f api/containers/docker-compose.yml up -d
	@echo "✓ Containers started"
	@echo ""
	@echo "Services:"
	@echo "  PostgreSQL: localhost:5435 (user: root, password: root, db: auction_db)"
	@echo "  Redis:      localhost:6379"
	@echo "  NATS:       localhost:4222 (monitoring: http://localhost:8222)"

docker-down:
	@echo "Stopping Docker containers..."
	docker-compose -f api/containers/docker-compose.yml down
	@echo "✓ Containers stopped"

docker-logs:
	docker-compose -f api/containers/docker-compose.yml logs -f

# Aplicar migraciones de base de datos
db-migrate:
	@echo "Applying database migrations..."
	psql postgresql://root:root@localhost:5435/auction_db -f api/sql/001_init.sql
	@echo "✓ Schema created"

db-seed:
	@echo "Seeding sample data..."
	psql postgresql://root:root@localhost:5435/auction_db -f api/sql/002_seed_data.sql
	@echo "✓ Sample data loaded"
