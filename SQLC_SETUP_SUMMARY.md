# SQLC Setup Summary

Configuración completada para SQLC y PostgreSQL en el proyecto "Can You Buy Me".

## Archivos Creados/Modificados

### Configuración SQLC
- ✅ `sqlc.yaml` - Configuración de SQLC (engine: postgresql, output: db/sqlc)

### Queries SQL
- ✅ `db/queries/users.sql` - Queries para usuarios (GetUserByEmail, CreateUser, etc.)
- ✅ `db/queries/auctions.sql` - Queries para subastas (GetAuction, ListAuctions, PlaceBid, etc.)
- ✅ `db/queries/bids.sql` - Queries para pujas (CreateBid, ListBidsByAuction, etc.)

### Paquete Database
- ✅ `internal/database/database.go` - Conexión a PostgreSQL con pgxpool + healthcheck

### Configuración
- ✅ `go.mod` - Actualizado con `github.com/jackc/pgx/v5` (reemplazo de `github.com/lib/pq`)
- ✅ `Makefile` - Agregado comando `make sqlc`
- ✅ `cmd/server/main.go` - Actualizado para conectar a PostgreSQL con verificación

### Documentación
- ✅ `DATABASE.md` - Guía completa de SQLC y database
- ✅ `SETUP.md` - Instrucciones actualizadas con SQLC
- ✅ `SERVER_README.md` - Actualizado con requisitos de SQLC

## Stack Actual

```
Frontend (Flutter Web)
         ↓ (gRPC-Web)
    HTTP Server (port 8080)
    gRPC Server (port 50051)
         ↓
    Handlers/Services
         ↓
    SQLC Queries (db/sqlc)
         ↓
    PostgreSQL (port 5435)
```

## Pasos Siguientes para Desarrollo

### 1. Instalar SQLC
```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

### 2. Descargar dependencias
```bash
go mod download
go mod tidy
```

### 3. Generar código
```bash
make sqlc  # Genera código Go desde db/queries/*.sql
```

### 4. Levantar servicios
```bash
make docker-up
make db-migrate
```

### 5. Compilar y ejecutar
```bash
make build
make run
```

## Queries Disponibles

> **Nota sobre las firmas:** la lista de abajo muestra los parámetros lógicos.
> En el código generado, las queries con **≥2 parámetros** reciben un struct
> `<Query>Params` (p.ej. `CreateUserParams`, `ListUsersParams`), NO argumentos
> posicionales. Las `:one` devuelven el struct **por valor** (`(User, error)`).

### Users
- `GetUserByID(ctx, id)` - Una fila
- `GetUserByEmail(ctx, email)` - Una fila
- `CreateUser(ctx, email, name, password_hash)` - Inserta y retorna
- `UpdateUserStripeCustomer(ctx, stripe_id, user_id)` - Update
- `ListUsers(ctx, limit, offset)` - Múltiples filas
- `DeactivateUser(ctx, id)` - Delete (soft)

### Auctions
- `GetAuctionByID(ctx, id)` - Una fila
- `ListActiveAuctions(ctx, limit, offset)` - Múltiples
- `ListAuctionsByStatus(ctx, status, limit, offset)` - Filtrado
- `CreateAuction(ctx, ...)` - Inserta
- `UpdateAuctionStatus(ctx, status, id)` - Update
- `UpdateAuctionStarted(ctx, id)` - Marca como active
- `UpdateAuctionClosed(ctx, id, winner_id, final_price)` - Cierra
- `UpdateCurrentHighestBid(ctx, bid, bidder_id, id)` - Actualiza puja mayor
- `GetAuctionForBidding(ctx, id)` - Validación de puja

### Bids
- `CreateBid(ctx, auction_id, user_id, amount, ip, user_agent)` - Inserta
- `GetBidByID(ctx, id)` - Una fila
- `ListBidsByAuction(ctx, auction_id, limit, offset)` - Historial
- `GetHighestBidForAuction(ctx, auction_id)` - Mayor puja
- `ListBidsByUser(ctx, user_id, limit, offset)` - Pujas del usuario
- `CountBidsByAuction(ctx, auction_id)` - Cuenta de pujas
- `InvalidateBid(ctx, reason, id)` - Marca como inválida

## Estructura de Código Generado

Cuando ejecutas `make sqlc`, genera:

```go
// db/sqlc/db.go
type Queries struct {
    db DBTX
}

func New(db DBTX) *Queries { ... }   // NO existe db.NewQueries()

// db/sqlc/models.go — columnas nullable → tipos pgtype, NO string/bool
type User struct {
    ID                     int32
    Email                  string
    Name                   string
    PasswordHash           string
    StripeCustomerID       pgtype.Text  // VARCHAR nullable
    StripePaymentMethodID  pgtype.Text  // VARCHAR nullable
    HasActivePaymentMethod pgtype.Bool  // BOOLEAN nullable
    CreatedAt              pgtype.Timestamp
    UpdatedAt              pgtype.Timestamp
    IsActive               pgtype.Bool
}

type Auction struct { ... }
type Bid struct { ... }

// db/sqlc/users.sql.go — :one devuelve el struct POR VALOR (no puntero)
func (q *Queries) GetUserByEmail(ctx context.Context, email string) (User, error) { ... }
// ≥2 parámetros → struct CreateUserParams, NO argumentos posicionales
func (q *Queries) CreateUser(ctx context.Context, arg CreateUserParams) (User, error) { ... }
...

// db/sqlc/auctions.sql.go
// db/sqlc/bids.sql.go
```

## Cómo Usar en Handlers

```go
package handlers

import (
    db "github.com/can-you-buy-me/db/sqlc"  // sqlc genera en db/sqlc/ (paquete db)
)

type UserHandler struct {
    queries *db.Queries
}

func NewUserHandler(database *db.DB) *UserHandler {
    return &UserHandler{
        queries: db.New(database),   // *db.DB satisface DBTX vía el pool embebido
    }
}

func (h *UserHandler) Register(ctx context.Context, email, name, password string) (db.User, error) {
    return h.queries.CreateUser(ctx, db.CreateUserParams{   // struct, no posicional
        Email:        email,
        Name:         name,
        PasswordHash: password,
    })
}

func (h *UserHandler) GetUser(ctx context.Context, id int32) (db.User, error) {
    return h.queries.GetUserByID(ctx, id)   // (User, error) por valor
}
```

## Verificación de Conectividad

main.go ahora hace esto automáticamente:

```go
// Conectar a PostgreSQL (variable dbConn, NO db: colisiona con el paquete db)
dbConn, err := database.New()
if err != nil {
    slog.Error("Failed to connect to database", "error", err)
    os.Exit(1)
}
defer dbConn.Close()

slog.Info("Database connection established",
    "host", "localhost",
    "port", "5435",
    "name", "auction_db",
)
```

## Próximas Mejoras

1. **Agregar más queries:**
   - `db/queries/payments.sql` - Stripe payment tracking
   - `db/queries/audit_log.sql` - Event logging

2. **Implementar servicios:**
   - AuthService (Register, Login con bcrypt)
   - AuctionService (GetAuction, ListAuctions, PlaceBid)
   - PaymentService (Stripe integration)

3. **Tests:**
   - Unit tests para queries (con testcontainers)
   - Integration tests para handlers

4. **Optimizaciones:**
   - Índices en columnas usadas frecuentemente
   - Prepared statements (sqlc maneja esto)
   - Connection pooling tuning

## Diferencias vs genx

El proyecto genx es más complejo:
- ✅ Múltiples roles de BD (app, admin)
- ✅ Row-Level Security (RLS)
- ✅ Context hooks para branch_id
- ✅ UUID overrides en SQLC

Este proyecto es MVP:
- ✅ Un usuario simple sin roles
- ✅ Sin RLS (solo validación en app)
- ✅ Simple queries sin context hooks
- ✅ Fácil de escalar después

## Referencias

- **DATABASE.md** - Guía completa de SQLC
- **SETUP.md** - Instrucciones de primer arranque
- **SERVER_README.md** - Cómo ejecutar el servidor
- **SQLC Docs** - https://docs.sqlc.dev/
