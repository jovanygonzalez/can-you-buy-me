# Can You Buy Me - Go Server

Servidor gRPC-Web para la plataforma de subastas en tiempo real.

## Requisitos

- **Go 1.22+**
- **protoc** (protobuf compiler)
- **sqlc** (SQL code generator)
- **Docker** (para PostgreSQL, Redis, NATS locales)

## Instalación

### 1. Instalar herramientas

#### Protobuf
```bash
# En Windows (usando Go)
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# O descargarlo desde: https://github.com/protocolbuffers/protobuf/releases
```

#### SQLC (SQL Code Generator)
```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
sqlc version
```

### 2. Descargar dependencias

```bash
go mod download
go mod tidy
```

## Compilación

### Generar código desde protos

```bash
make proto
# O manualmente:
# protoc --go_out=. --go-grpc_out=. proto/v1/*.proto
```

### Compilar el servidor

```bash
make build
# Genera: bin/server
```

## Ejecución

### Opción 1: Ejecutar localmente (con Docker)

```bash
# Terminal 1: Iniciar servicios Docker
make docker-up

# Terminal 2: Aplicar migraciones de BD
make db-migrate

# Terminal 3: Cargar datos de ejemplo (opcional)
make db-seed

# Terminal 4: Ejecutar el servidor
make run
```

El servidor estará disponible en:
- **gRPC:** `localhost:50051`
- **gRPC-Web (HTTP):** `localhost:8080`
- **Health Check:** `curl http://localhost:8080/health`

### Opción 2: Ejecutar el binario compilado

```bash
make build
./bin/server
```

## Estructura del Proyecto

```
/cmd/server/
  └── main.go              # Punto de entrada del servidor

/internal/
  ├── grpc/
  │   └── server.go        # Servidor gRPC + gRPC-Web (sin Envoy)
  ├── handlers/
  │   ├── health.go        # HealthService
  │   ├── auth.go          # AuthService (Register/Login)
  │   └── payment.go       # PaymentService (InitializeStripePayment)
  ├── middleware/
  │   └── auth.go          # Interceptor JWT
  ├── security/
  │   └── jwt.go           # JWTManager (HS256, 24h)
  ├── database/
  │   └── database.go      # pgxpool + ping al arranque
  ├── payment/
  │   └── stripe.go        # Cliente Stripe (Customer, SetupIntent)
  └── webhook/
      └── stripe.go        # Handler HTTP /webhooks/stripe

/db/
  ├── queries/             # *.sql fuente para sqlc
  └── sqlc/                # (generado por sqlc)

/proto/v1/
  ├── health.proto
  ├── auth.proto           # Auth (registro/login)
  ├── auction.proto        # Auction (pujas) ✓
  └── payment.proto        # Payment (Stripe Setup Intent)

/pkg/gen/
  └── (generado automáticamente desde los .proto)

/api/
  ├── containers/          # Dockerfile y docker-compose
  └── sql/                 # Schema (001_init) + seed (002)
```

## Servicios gRPC

### HealthService
- `Ping(PingRequest) -> PingResponse` - Verificar que el servidor está vivo
- `Check(HealthCheckRequest) -> HealthCheckResponse` - Verificar salud del servidor

### AuthService ✓
- `Register(RegisterRequest) -> RegisterResponse`
- `Login(LoginRequest) -> LoginResponse`

### PaymentService ✓
- `InitializeStripePayment(InitializeStripePaymentRequest) -> InitializeStripePaymentResponse`
- Webhook HTTP: `POST /webhooks/stripe` (setup_intent.succeeded)

### AuctionService ✓
- `GetAuction(GetAuctionRequest) -> Auction` — snapshot (catálogo + precio en vivo del KV)
- `ListAuctions(ListAuctionsRequest) -> ListAuctionsResponse`
- `PlaceBid(PlaceBidRequest) -> PlaceBidResponse` — valida con CAS sobre NATS KV y publica en `auction.<id>.bids`
- Pujas → Postgres de forma asíncrona vía audit consumer (`internal/messaging`)
- Tiempo real: clientes se suscriben por NATS Core sobre WebSocket (`:8443`)

## Probar el servidor

### Con HTTP/Health Check
```bash
curl http://localhost:8080/health
```

### Con grpcurl (herramienta gRPC)
```bash
# Instalar grpcurl
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

# Listar servicios disponibles
grpcurl -plaintext localhost:50051 list

# Probar Ping
grpcurl -plaintext localhost:50051 health.v1.HealthService/Ping
```

### Con Flutter Web (futuro)
El cliente Flutter Web se conectará vía gRPC-Web a `http://localhost:8080`.

## Variables de Entorno

Ver `.env.example` para la lista completa. Configurar antes de ejecutar:

```bash
cp .env.example .env
# Editar .env con tus valores
```

### Principales para desarrollo:
- `GRPC_PORT=50051` - Puerto del servidor gRPC
- `HTTP_PORT=8080` - Puerto del servidor HTTP (gRPC-Web)
- `DB_HOST=localhost`
- `DB_PORT=5435`
- `REDIS_URL=redis://localhost:6379/0`
- `NATS_URL=nats://localhost:4222`

## Tecnología Usada

### Sin Envoy
Este servidor usa `github.com/improbable-eng/grpc-web` que envuelve directamente el servidor gRPC estándar de Go. **No requiere Envoy proxy**.

```
Flutter Web Client
        ↓ (HTTP/1.1 + gRPC-Web headers)
    gRPC-Web Handler (improbable-eng/grpc-web)
        ↓
    gRPC Server (google.golang.org/grpc)
        ↓
    Handlers (Auth, Auction, Health, etc)
```

### Stack
- **gRPC:** Comunicación eficiente en bajo latency
- **gRPC-Web:** Acceso desde navegador (HTTP/1.1)
- **PostgreSQL:** Base de datos principal
- **Redis:** Cache de catálogo
- **NATS JetStream:** Pub/sub para pujas en tiempo real

## Próximos Pasos (MVP)

1. ✅ Estructura básica con Health Check
2. ✅ Conectar a PostgreSQL (pgx/v5 + sqlc, ping al arranque)
3. ✅ Implementar AuthService (Registro/Login con bcrypt + JWT)
4. ✅ Integración con Stripe Setup Intents (PaymentService + webhook)
5. ✅ Implementar AuctionService (PlaceBid con CAS sobre NATS KV + GetAuction)
6. ✅ PlaceBid publica en NATS JetStream + audit consumer → Postgres (async)
7. ✅ Listener WebSocket de NATS habilitado (`:8443`, server-side)
8. ⏳ Cliente Flutter/Dart de NATS sobre WebSocket (suscripción a pujas)
9. ⏳ Endpoint admin `/admin/close-auction` (publica `auction.<id>.control`)
10. ⏳ Catálogo cacheado en Redis (hoy se lee de Postgres)
11. ⏳ Seguridad NATS prod (TLS + auth de subjects)

## Notas de Desarrollo

- **Logging:** Usa `log/slog` (logger estándar de Go 1.21+)
- **Graceful Shutdown:** El servidor detiene los servicios en orden al recibir SIGINT/SIGTERM
- **Reflection:** Habilitada para `grpcurl` y herramientas similares
- **Máximo tamaño de mensaje:** 10 MiB (para soportar payloads grandes en futuro)

## Troubleshooting

### Error: "protoc not found"
```bash
# Instalar protoc globalmente desde:
# https://github.com/protocolbuffers/protobuf/releases

# O en Windows con Chocolatey:
choco install protoc
```

### Error: "Failed to connect to database"
```bash
# Verificar que Docker está corriendo
make docker-up

# Ver logs de PostgreSQL
make docker-logs
```

### Error: "Address already in use"
```bash
# Cambiar puertos en .env
# O matar el proceso en el puerto:
# Windows: netstat -ano | findstr :8080
# Linux: lsof -i :8080
```
