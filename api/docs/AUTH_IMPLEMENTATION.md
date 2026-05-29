# AuthService Implementation

Referencia del AuthService (registro, login y JWT) — **ya implementado y cableado**.
Los "pasos" de abajo son para regenerar código y arrancar, no para completar algo pendiente.

## Arquitectura

```
Flutter Web (Cliente)
    ↓ (gRPC-Web)
AuthService
    ├─ Register(email, name, password)
    │   ├─ Validar entrada
    │   ├─ Hashear password con bcrypt
    │   ├─ INSERT usuario en PostgreSQL (sqlc)
    │   └─ Retornar RegisterResponse
    │
    └─ Login(email, password)
        ├─ Validar entrada
        ├─ SELECT usuario por email (sqlc)
        ├─ Comparar password con bcrypt
        ├─ Generar JWT token (24h)
        └─ Retornar LoginResponse + token
```

## Archivos Creados

### 1. Proto Definition
- **`proto/v1/auth.proto`** - Definición del servicio gRPC
  - `Register` - Crea nuevo usuario
  - `Login` - Autentica y retorna JWT

### 2. Seguridad
- **`internal/security/jwt.go`** - Manejo de JWT
  - `GenerateToken()` - Genera JWT signed con HS256
  - `ValidateToken()` - Valida JWT y extrae claims
  - `ExtractUserID()` - Obtiene user_id del token

### 3. Base de Datos
- **`db/queries/users.sql`** - Queries para usuarios
  - `GetUserByEmail()` - Obtiene usuario por email
  - `CreateUser()` - Inserta nuevo usuario
  - `UpdateUserStripeCustomer()` - Actualiza Stripe customer ID

### 4. Handlers
- **`internal/handlers/auth.go`** - Implementación del servicio
  - `Register()` - Registra usuario con password hash
  - `Login()` - Autentica y genera JWT

### 5. Middleware
- **`internal/middleware/auth.go`** - Validación de JWT para otros servicios
  - `AuthInterceptor()` - Interceptor gRPC que valida tokens
  - `GetUserIDFromContext()` - Extrae user_id del context

### 6. Dependencies
- **`go.mod`** - Actualizado con:
  - `github.com/golang-jwt/jwt/v5` - JWT tokens
  - `golang.org/x/crypto` - Bcrypt (ya incluido)

## Pasos para Completar

### Paso 1: Generar Código desde Protos y Queries

```bash
# Generar código de protos
make proto

# Generar código de sqlc
make sqlc

# Verificar que se generó:
ls -la pkg/gen/auth/v1/      # Debe existir
ls -la db/sqlc/              # Debe contener users.sql.go
```

### Paso 2: Descargar Dependencias

```bash
go mod download
go mod tidy
```

### Paso 3: AuthService ya está cableado en main.go

No hay que descomentar nada — ya está activo en `cmd/server/main.go`:

```go
// Crear queries para base de datos (variable dbConn, NO db: colisiona con el paquete)
queries := db.New(dbConn)

// Registrar AuthService
authHandler := handlers.NewAuthHandler(queries, jwtManager)
authpb.RegisterAuthServiceServer(grpcServer, authHandler)
```

### Paso 4: Middleware de Autenticación (ya cableado)

El interceptor JWT ya se pasa al crear el servidor gRPC (`internal/grpc/server.go`
acepta `extraOpts`), así que todos los RPC salvo los públicos quedan protegidos:

```go
grpcServer, grpcListener, err := grpcpkg.NewGRPCServer(
    grpcConfig,
    grpc.ChainUnaryInterceptor(
        middleware.AuthInterceptor(jwtManager),
    ),
)
```

### Paso 5: Compilar y Probar

```bash
make build
make run
```

## Probar el AuthService

### Con grpcurl

#### Registrar usuario
```bash
grpcurl -plaintext \
  -d '{
    "email": "user@example.com",
    "name": "John Doe",
    "password": "password123"
  }' \
  localhost:50051 auth.v1.AuthService/Register
```

Respuesta esperada:
```json
{
  "userId": 1,
  "email": "user@example.com",
  "message": "User registered successfully"
}
```

#### Login
```bash
grpcurl -plaintext \
  -d '{
    "email": "user@example.com",
    "password": "password123"
  }' \
  localhost:50051 auth.v1.AuthService/Login
```

Respuesta esperada:
```json
{
  "jwtToken": "eyJhbGc...",
  "userId": 1,
  "email": "user@example.com"
}
```

### Usar Token en Otros Servicios

```bash
grpcurl -plaintext \
  -H "authorization: Bearer eyJhbGc..." \
  -d '{}' \
  localhost:50051 health.v1.HealthService/Check
```

## Seguridad - Implementado

✅ **Hashing de Contraseñas**
- Bcrypt con cost=10 (default)
- Salt automático incluido
- Nunca guardar plaintext

✅ **JWT Tokens**
- Algoritmo: HS256 (HMAC-SHA256)
- Expiración: 24 horas
- Claims: user_id, email, iat, exp, nbf, iss

✅ **Validación de Token**
- Verificación de firma
- Validación de expiración
- Extracción segura de claims

✅ **Métodos Públicos**
- Register y Login no requieren autenticación
- Otros servicios requieren JWT válido

## Próximos Pasos

### 1. Refresh Tokens (Futuro)
```go
// Agregar en auth.proto
rpc RefreshToken(RefreshTokenRequest) returns (RefreshTokenResponse);

// Implementar rotación de tokens
// JWT actual: 24h
// Refresh token: 30 días
```

### 2. Email Verification (Futuro)
```sql
-- Agregar a users table
ALTER TABLE users ADD COLUMN email_verified BOOLEAN DEFAULT false;
ALTER TABLE users ADD COLUMN verification_token VARCHAR(255);
```

### 3. Password Reset (Futuro)
```sql
-- Agregar campos para reset
ALTER TABLE users ADD COLUMN reset_token VARCHAR(255);
ALTER TABLE users ADD COLUMN reset_token_expires_at TIMESTAMP;
```

### 4. Session Tracking (Futuro)
```sql
-- Tabla para sessions
CREATE TABLE sessions (
  id BIGSERIAL PRIMARY KEY,
  user_id INT NOT NULL REFERENCES users(id),
  token_jti VARCHAR(255) UNIQUE,  -- JWT ID
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  expires_at TIMESTAMP
);
```

## Variables de Entorno

En `.env`:
```bash
JWT_SECRET=your-super-secret-key-change-in-production-min-32-chars
```

**Importante:** El secret debe ser:
- Al menos 32 caracteres en producción
- Único y seguro (generar con: `openssl rand -base64 32`)
- NO commitar en git

## Testing

### Unit Tests (Ejemplo)

```go
func TestRegister(t *testing.T) {
    handler := NewAuthHandler(mockQueries, jwtManager)
    
    resp, err := handler.Register(context.Background(), &pb.RegisterRequest{
        Email:    "test@example.com",
        Name:     "Test User",
        Password: "password123",
    })
    
    assert.NoError(t, err)
    assert.NotNil(t, resp)
    assert.Equal(t, "test@example.com", resp.Email)
}
```

### Integration Tests (Ejemplo)

```bash
# Usar testcontainers para PostgreSQL en tests
# Ver: https://testcontainers.com/
```

## Troubleshooting

### Error: "undefined: db.New"
- Ejecutar: `make sqlc`
- Verificar que `db/sqlc/db.go` existe

### Error: "user with this email already exists"
- Expected - validación correcta
- Usar otro email para registrar

### Error: "invalid token"
- El JWT expiró (después de 24h)
- Hacer login nuevamente

### Error: "missing authorization token"
- Agregar header: `authorization: Bearer <token>`

## Referencia Rápida

| Operación | Comando |
|-----------|---------|
| Registrar | `grpcurl ... auth.v1.AuthService/Register` |
| Login | `grpcurl ... auth.v1.AuthService/Login` |
| Validar token | Se valida automáticamente con interceptor |
| Generar secret | `openssl rand -base64 32` |

## Referencias

- **JWT:** https://jwt.io/
- **Bcrypt:** https://pkg.go.dev/golang.org/x/crypto/bcrypt
- **golang-jwt:** https://github.com/golang-jwt/jwt
- **gRPC Security:** https://grpc.io/docs/guides/auth/
