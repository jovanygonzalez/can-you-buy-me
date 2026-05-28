# Arquitectura del Servidor Go

## Diagrama General

```
┌─────────────────────────────────────────────────────────────┐
│                        FRONTEND (Flutter Web)                │
│                     http://localhost:3000                    │
└────────────────────────────┬────────────────────────────────┘
                             │
                  gRPC-Web (HTTP/1.1)
                 (improbable-eng/grpc-web)
                             │
                             ▼
        ┌────────────────────────────────────────┐
        │    HTTP Server (go/net http)           │
        │    localhost:8080                      │
        ├────────────────────────────────────────┤
        │ /health                    (HTTP GET)  │
        │ /webhooks/stripe           (HTTP POST) │
        │ /                          (gRPC-Web)  │
        └────────────────────────────┬───────────┘
                                     │
                         ┌───────────┴───────────┐
                         │                       │
                         ▼                       ▼
            ┌─────────────────────┐  ┌──────────────────────┐
            │  gRPC Server        │  │  HTTP Handlers       │
            │  localhost:50051    │  │  /health             │
            ├─────────────────────┤  │  /webhooks/stripe    │
            │ HealthService    ✓  │  └──────────────────────┘
            │ AuthService      ✓  │
            │ PaymentService   ✓  │
            │ AuctionService TODO │
            └─────────┬───────────┘
                      │
         ┌────────────┼────────────┐
         │            │            │
         ▼            ▼            ▼
    ┌────────┐  ┌────────┐  ┌──────────┐
    │  DB    │  │ Redis  │  │   NATS   │
    │ Postgres│  │ Cache  │  │ JetStream│
    │ :5435  │  │ :6379  │  │  :4222   │
    └────────┘  └────────┘  └──────────┘
```

## Flujo de una Puja (MVP)

```
1. Usuario hace una puja
   Flutter Web
   │
   └─→ gRPC-Web: PlaceBid(auction_id=1, bid_amount=100)
       │
       ▼
   Server Go
   │
   ├─→ Validar puja contra base de datos
   │   └─→ PostgreSQL: SELECT current_highest_bid FROM auctions WHERE id=1
   │
   ├─→ Publicar en NATS JetStream
   │   └─→ Publish("auctions.1.bids", {user_id, bid_amount, timestamp})
   │
   ├─→ Guardar en PostgreSQL (auditoría)
   │   └─→ INSERT INTO bids (auction_id, user_id, bid_amount, ...)
   │
   └─→ Retornar PlaceBidResponse(success=true)
       │
       ▼
   Flutter Web (todas las instancias)
   │
   └─→ WebSocket listener a NATS
       └─→ Reciben evento: {user_id, bid_amount, timestamp}
           └─→ Actualizan UI con el precio más alto

2. Admin cierra subasta manualmente (POST /admin/close-auction)
   │
   ├─→ UPDATE auctions SET status='closed' WHERE id=1
   ├─→ Publish("auctions.1.closed", {winner_id, final_price})
   └─→ Flutter cierra el formulario de pujas
```

## Componentes Principales

### 1. HTTP Server (go/net/http)
- **Puerto:** 8080
- **Responsabilidades:**
  - Servir gRPC-Web (sin Envoy)
  - Health check endpoint (/health)
- **Dependencias:** `github.com/improbable-eng/grpc-web`

```go
// Envuelve el gRPC server
grpcWebHandler := grpcweb.WrapServer(grpcServer)
httpMux.Handle("/", grpcWebHandler)
```

### 2. gRPC Server
- **Puerto:** 50051
- **Responsabilidades:**
  - Servir RPCs para gRPC-Web y gRPC nativo
  - Reflection (grpcurl, debugging)
- **Máximo tamaño de mensaje:** 10 MiB

### 3. Handlers (RPCs)

#### HealthService
```protobuf
service HealthService {
  rpc Ping(PingRequest) returns (PingResponse);
  rpc Check(HealthCheckRequest) returns (HealthCheckResponse);
}
```

Implementado en: `internal/handlers/health.go`

#### AuthService ✓ (implementado)
```protobuf
service AuthService {
  rpc Register(RegisterRequest) returns (RegisterResponse);
  rpc Login(LoginRequest) returns (LoginResponse);
}
```

Implementado en: `internal/handlers/auth.go`
- Bcrypt hashing (`golang.org/x/crypto/bcrypt`)
- JWT HS256, 24h, issuer `auction-api` (`internal/security/jwt.go`)
- Persistencia vía sqlc + pgx/v5

#### PaymentService ✓ (implementado)
```protobuf
service PaymentService {
  rpc InitializeStripePayment(InitializeStripePaymentRequest) returns (InitializeStripePaymentResponse);
}
```

Implementado en: `internal/handlers/payment.go`
- Crea (idempotente) un Stripe Customer y un Setup Intent
- Retorna `client_secret` para que Flutter confirme la tarjeta con Stripe.js
- Solo se registra si `STRIPE_API_KEY` está configurada (degradación elegante)

#### Webhook de Stripe ✓ (implementado, HTTP no gRPC)
`POST /webhooks/stripe` — `internal/webhook/stripe.go`
- Verifica firma `Stripe-Signature` con `webhook.ConstructEvent()`
- Maneja `setup_intent.succeeded` / `setup_intent.setup_failed`
- MVP Fase 1: solo loggea. Fase 2: actualizará `has_active_payment_method` en BD.

#### Middleware de autenticación ✓
`internal/middleware/auth.go` — `UnaryServerInterceptor` que valida el JWT
en todos los RPC salvo los públicos (Register, Login, Health Ping/Check) e
inyecta `user_id`/`email` en el `context`.

#### AuctionService (TODO)
```protobuf
service AuctionService {
  rpc GetAuction(GetAuctionRequest) returns (Auction);
  rpc ListAuctions(ListAuctionsRequest) returns (ListAuctionsResponse);
  rpc PlaceBid(PlaceBidRequest) returns (PlaceBidResponse);
}
```

A implementar:
- Leer catálogo de Redis
- Validar pujas contra PostgreSQL
- Publicar en NATS JetStream

## Integración con Servicios Externos

### PostgreSQL ✓ (implementado)
```go
// internal/database/database.go — pgxpool + ping de conectividad al arranque
dbConn, err := database.New()   // lee DB_* de env, hace pool.Ping()
queries := db.New(dbConn)       // queries type-safe generadas por sqlc (pgx/v5)
```
Driver: `github.com/jackc/pgx/v5` (NO `lib/pq`). Queries generadas por sqlc en `db/sqlc/`.

### Redis
```go
// TODO: Implementar cliente de Redis para cachear catálogo
// redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
```

### NATS JetStream
```go
// TODO: Implementar publicador para pujas en tiempo real
// nc, _ := nats.Connect("nats://localhost:4222")
// js, _ := nc.JetStream()
// js.Publish("auctions.1.bids", bidData)
```

### Stripe ✓ (implementado — Setup Intents)
```go
// internal/payment/stripe.go — verifica la API key al arranque (account.GetByID)
// PaymentService.InitializeStripePayment → Customer + SetupIntent (off_session)
// internal/webhook/stripe.go → recibe setup_intent.succeeded vía webhook
```
Charging manual desde el dashboard de Stripe en el MVP (no se automatiza).

## Estructura de Archivos

```
cmd/server/
  └── main.go                    # Entry point + setup

internal/
  ├── grpc/
  │   └── server.go              # gRPC server factory + gRPC-Web (sin Envoy)
  ├── handlers/
  │   ├── health.go              # HealthService ✓
  │   ├── auth.go                # AuthService ✓ (Register/Login)
  │   ├── payment.go             # PaymentService ✓ (InitializeStripePayment)
  │   └── auction.go (TODO)      # AuctionService
  ├── middleware/
  │   └── auth.go                # Interceptor JWT (rutas públicas + context)
  ├── security/
  │   └── jwt.go                 # JWTManager (HS256, 24h)
  ├── database/
  │   └── database.go            # pgxpool + ping al arranque
  ├── payment/
  │   └── stripe.go              # Cliente Stripe (Customer, SetupIntent)
  └── webhook/
      └── stripe.go              # Handler HTTP /webhooks/stripe

db/                               # sqlc
  ├── queries/                   # *.sql (fuente de queries)
  │   ├── users.sql
  │   ├── auctions.sql
  │   └── bids.sql
  └── sqlc/                      # (Generado: New(), Queries, structs)

proto/v1/
  ├── health.proto
  ├── auth.proto
  ├── auction.proto
  └── payment.proto

pkg/gen/                          # (Generado automáticamente por protoc)
  ├── health/v1/
  ├── auth/v1/
  ├── auction/v1/
  └── payment/v1/

api/
  ├── containers/
  │   ├── Dockerfile             # PostgreSQL
  │   └── docker-compose.yml     # Todos los servicios
  └── sql/
      ├── 001_init.sql           # Schema (fuente única — sqlc lo lee de aquí)
      └── 002_seed_data.sql      # Datos de ejemplo
```

## Stack de Dependencias

```
main.go
├── github.com/joho/godotenv             (Config desde .env)
├── log/slog                              (Logging)
├── google.golang.org/grpc                (gRPC server)
├── google.golang.org/grpc/reflection
├── github.com/improbable-eng/grpc-web    (gRPC-Web sin Envoy)
├── github.com/jackc/pgx/v5 (+pgxpool)    (PostgreSQL driver — NO lib/pq)
├── github.com/golang-jwt/jwt/v5          (JWT HS256)
├── golang.org/x/crypto/bcrypt            (Hash de contraseñas)
├── github.com/stripe/stripe-go/v76       (Stripe Customer + SetupIntent + webhook)
└── (generado) db/sqlc                    (sqlc) + pkg/gen (protoc)

(En go.mod, aún SIN usar en código — wiring pendiente)
├── github.com/redis/go-redis/v9          (Redis — catálogo, TODO)
└── github.com/nats-io/nats.go            (NATS JetStream — pujas, TODO)
```

## Graceful Shutdown

```
SIGINT/SIGTERM
    │
    ├─→ Parar HTTP server (timeout 10s)
    ├─→ Parar gRPC server (cerrar conexiones)
    └─→ Cerrar recursos (DB, Redis, NATS)
```

## Monitoreo & Debugging

### gRPC Reflection
```bash
grpcurl -plaintext localhost:50051 list
grpcurl -plaintext -d '{}' localhost:50051 health.v1.HealthService/Ping
```

### Health Endpoint HTTP
```bash
curl http://localhost:8080/health
```

### Logs
```
INFO Starting Can You Buy Me API Server...
INFO Server configuration grpc_port=50051 http_port=8080
INFO gRPC server listening port=50051
INFO HTTP/gRPC-Web server listening port=8080
INFO Health check successful
```

## Escalabilidad (Post-MVP)

1. **Load Balancer:** Múltiples instancias de Go server
2. **Message Queue:** NATS clustering (3+ nodos)
3. **Database:** PostgreSQL replication
4. **Cache:** Redis cluster
5. **Observabilidad:** OpenTelemetry, Prometheus, Jaeger

Arquitectura lista para Event Sourcing + CQRS en futuro.
