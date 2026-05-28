# Database Setup - SQLC & PostgreSQL

Guía para trabajar con la base de datos usando SQLC (SQL Code Generator).

## Qué es SQLC

SQLC es un compilador que toma queries SQL y genera código Go type-safe sin ORM.

**Flujo:**
```
.sql files (db/queries/*.sql)
         ↓
    sqlc compiler
         ↓
Generated Go code (db/sqlc/*.go)
         ↓
Use in handlers/services
```

## Estructura de Carpetas

```
api/sql/                   (SCHEMA + seed — fuente de verdad)
├── 001_init.sql           (schema; sqlc lo lee + lo aplica db-migrate)
└── 002_seed_data.sql      (datos de ejemplo)

db/
├── queries/               (queries fuente para sqlc)
│   ├── users.sql
│   ├── auctions.sql
│   └── bids.sql
└── sqlc/                  (GENERADO — no editar a mano)
    ├── db.go              (interface DBTX + New())
    ├── models.go          (structs: User, Auction, Bid, ...)
    ├── users.sql.go
    ├── auctions.sql.go
    └── bids.sql.go
```

## Instalación de SQLC

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
sqlc version
```

## Configuración (sqlc.yaml)

Ya está configurado en la raíz del proyecto:

```yaml
version: "2"
sql:
  - engine: "postgresql"
    queries: "db/queries"
    schema: "api/sql/001_init.sql"   # fuente única de schema (misma que aplica db-migrate)
    gen:
      go:
        package: "db"
        out: "db/sqlc"
        sql_package: "pgx/v5"  # Usar pgx directamente
        emit_json_tags: true
        emit_interface: true
```

## Cómo Escribir Queries en SQLC

### Sintaxis Básica

```sql
-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1;
```

**Componentes:**
- `-- name: GetUserByEmail` - Nombre de la función generada
- `:one` - Modo de retorno (one, many, exec, exec_result)
- `$1` - Parámetro (POSIX, no `:param`)

### Modos de Retorno

| Modo | Retorna | Ejemplo |
|------|---------|---------|
| `:one` | `(User, error)` ⚠️ **por valor, no puntero** | `SELECT * FROM users WHERE id = $1` |
| `:many` | `([]User, error)` | `SELECT * FROM users` |
| `:exec` | `error` | `DELETE FROM users WHERE id = $1` |
| `:exec_result` | `(pgconn.CommandTag, error)` | Retorna filas afectadas |

> ⚠️ **`:one` devuelve el struct por valor**, NO un puntero. En "no encontrado"
> devuelve `(User{}, pgx.ErrNoRows)`. **Nunca** compares el resultado con `nil`
> (`if user == nil` no compila) — chequea `err`.

### Ejemplos

#### SELECT (una fila)
```sql
-- name: GetUserByID :one
SELECT id, email, name, created_at FROM users
WHERE id = $1;
```

Genera (1 parámetro → argumento directo, struct **por valor**):
```go
func (q *Queries) GetUserByID(ctx context.Context, id int32) (User, error) { ... }
```

#### SELECT (múltiples filas)
```sql
-- name: ListAuctions :many
SELECT * FROM auctions
WHERE status = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
```

Genera (⚠️ **≥2 parámetros → struct `...Params`, NO argumentos posicionales**):
```go
type ListAuctionsParams struct {
    Status string
    Limit  int32
    Offset int32
}
func (q *Queries) ListAuctions(ctx context.Context, arg ListAuctionsParams) ([]Auction, error) { ... }
```

#### INSERT
```sql
-- name: CreateUser :one
INSERT INTO users (email, name, password_hash, created_at, updated_at)
VALUES ($1, $2, $3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
RETURNING *;
```

Genera (struct `CreateUserParams`, resultado **por valor**):
```go
type CreateUserParams struct {
    Email        string
    Name         string
    PasswordHash string
}
func (q *Queries) CreateUser(ctx context.Context, arg CreateUserParams) (User, error) { ... }
```

> 💡 **Columnas nullable → tipos `pgtype`.** Como `stripe_customer_id` es
> `VARCHAR(255)` sin `NOT NULL`, sqlc lo genera como `pgtype.Text` (no `string`).
> Para leerlo: `if u.StripeCustomerID.Valid { ... u.StripeCustomerID.String }`.
> Para escribirlo: `pgtype.Text{String: id, Valid: true}`. Igual con
> `pgtype.Bool`, `pgtype.Timestamp`, `pgtype.Numeric` (DECIMAL), etc.

#### UPDATE
```sql
-- name: UpdateAuctionStatus :one
UPDATE auctions
SET status = $1, updated_at = CURRENT_TIMESTAMP
WHERE id = $2
RETURNING *;
```

#### DELETE
```sql
-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1;
```

Genera:
```go
func (q *Queries) DeleteUser(ctx context.Context, id int32) error { ... }
```

### Parámetros Nombrados

Para mayor claridad en INSERTs/UPDATEs complejos, usa parámetros nombrados:

```sql
-- name: UpsertSetting :one
INSERT INTO settings (key, value, updated_by)
VALUES (@key, @value, @user_id)
ON CONFLICT (key) DO UPDATE SET
    value = EXCLUDED.value,
    updated_by = @user_id
RETURNING *;
```

Se llama con:
```go
q.UpsertSetting(ctx, db.UpsertSettingParams{
    Key:    "setting_key",
    Value:  "value",
    UserID: 123,
})
```

## Generar Código

```bash
# Automático (en make build)
make build

# Manual
make sqlc

# O directamente
sqlc generate
```

Verifica que aparecen archivos en `db/sqlc/`:
```bash
ls -la db/sqlc/
```

## Usar en Handlers/Services

### Estructura Típica

```go
package handlers

import (
    "context"
    db "github.com/can-you-buy-me/db/sqlc"  // sqlc genera en db/sqlc/ (paquete db)
)

type UserHandler struct {
    queries *db.Queries
}

func NewUserHandler(queries *db.Queries) *UserHandler {
    return &UserHandler{queries: queries}
}

func (h *UserHandler) GetUser(ctx context.Context, userID int32) (db.User, error) {
    return h.queries.GetUserByID(ctx, userID) // (User, error) por valor
}
```

### Conectar en main.go

```go
import "github.com/can-you-buy-me/internal/database"

func main() {
    // Conectar a PostgreSQL (*database.DB envuelve *pgxpool.Pool)
    dbConn, err := database.New()
    if err != nil {
        slog.Error("Failed to connect", "error", err)
        os.Exit(1)
    }
    defer dbConn.Close()

    // sqlc genera New(DBTX) *Queries — *database.DB satisface DBTX
    // (vía el *pgxpool.Pool embebido). NO existe db.NewQueries().
    queries := db.New(dbConn)

    // Pasar las queries a los handlers
    userHandler := handlers.NewUserHandler(queries)
}
```

## Ciclo de Desarrollo

1. **Modificar schema:**
   ```bash
   # Editar api/sql/001_init.sql
   # Aplicar cambios a la BD local
   make db-migrate
   ```

2. **Escribir query:**
   ```bash
   # Editar db/queries/users.sql
   # Agregar nueva query con -- name: ...
   ```

3. **Generar código:**
   ```bash
   make sqlc
   ```

4. **Usar en código:**
   ```go
   user, err := queries.GetUserByID(ctx, 1)
   ```

5. **Compilar:**
   ```bash
   make build
   ```

## Troubleshooting

### Error: "no matching sql type for go type"
- SQLC no entiende el tipo de columna
- Solución: Agregar override en `sqlc.yaml`

```yaml
overrides:
  - db_type: "uuid"
    go_type:
      import: "github.com/google/uuid"
      type: "UUID"
```

### Error: "query does not match schema"
- La query referencia una tabla/columna que no existe
- Solución: Verificar que la BD tiene el schema actualizado

```bash
psql postgresql://root:root@localhost:5435/auction_db -c "\dt"
```

### Error: "unrecognized mode"
- Válidos: `:one`, `:many`, `:exec`, `:exec_result`
- Verificar sintaxis: `-- name: FunctionName :mode`

### Cambios no aparecen después de `make sqlc`
- `db/sqlc/` es generado; los cambios se sobrescriben
- Editar `db/queries/*.sql`, no `db/sqlc/*.go`

## Mejores Prácticas

1. **Queries simples y atómicas:**
   ```sql
   -- ✓ Bien
   -- name: GetUserByID :one
   SELECT * FROM users WHERE id = $1;

   -- ✗ Evitar: Lógica compleja en SQL
   -- name: GetUserWithAllData :one
   SELECT ... FROM users
   LEFT JOIN auctions ...
   LEFT JOIN bids ...
   ```

2. **Nombres descriptivos:**
   ```sql
   -- ✓ Bien
   -- name: ListAuctionsByStatus :many

   -- ✗ Evitar: Nombres genéricos
   -- name: List :many
   ```

3. **Usar RETURNING en inserts/updates:**
   ```sql
   -- ✓ Bien
   INSERT INTO users (...) VALUES (...) RETURNING *;

   -- ✗ Evitar: Sin RETURNING
   INSERT INTO users (...) VALUES (...);
   ```

4. **Separar por dominio:**
   ```
   db/queries/
   ├── users.sql       (auth, profile)
   ├── auctions.sql    (catalog, state)
   ├── bids.sql        (bidding)
   └── payments.sql    (Stripe)
   ```

## Próximos Pasos

1. Actualizar schema: agregar fk, índices, constraints
2. Escribir más queries para AuctionService y PaymentService
3. Implementar repositories que usan sqlc
4. Tests: usar transactions para rollback automático

## Referencias

- **SQLC Docs:** https://docs.sqlc.dev/
- **PostgreSQL:** https://www.postgresql.org/docs/
- **pgx/v5:** https://github.com/jackc/pgx
