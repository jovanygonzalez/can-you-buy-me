# AuthService Implementation Summary

## ✅ Completado

He implementado un **AuthService completo** con Registro, Login y JWT tokens.

### 1. Protocolo gRPC (`proto/v1/auth.proto`)
```protobuf
service AuthService {
  rpc Register(RegisterRequest) returns (RegisterResponse);
  rpc Login(LoginRequest) returns (LoginResponse);
}
```

**Register:**
- Input: email, name, password
- Output: user_id, email, message
- Validación: email/name/password requeridos, password ≥6 caracteres
- Seguridad: Bcrypt hash, validar no-duplicado

**Login:**
- Input: email, password
- Output: JWT token, user_id, email
- Validación: email/password requeridos
- Seguridad: Comparar hash, generar JWT signed

### 2. Seguridad (`internal/security/jwt.go`)

**JWT Tokens:**
- Algoritmo: HS256 (HMAC-SHA256)
- Expiración: 24 horas
- Claims: user_id, email, iat, exp, nbf, iss
- Métodos:
  - `GenerateToken(userID, email)` → token string
  - `ValidateToken(tokenString)` → claims + error
  - `ExtractUserID(tokenString)` → user_id + error

**Ejemplo Claims:**
```json
{
  "user_id": 1,
  "email": "user@example.com",
  "iat": 1700000000,
  "exp": 1700086400,
  "nbf": 1700000000,
  "iss": "auction-api"
}
```

### 3. Base de Datos (`db/queries/users.sql`)

**Queries** (sqlc: `:one` → struct **por valor**; ≥2 params → struct `...Params`):
- `GetUserByEmail(ctx, email)` → `(User, error)`
- `CreateUser(ctx, CreateUserParams{...})` → `(User, error)`
- `UpdateUserStripeCustomer(ctx, UpdateUserStripeCustomerParams{...})` → `(User, error)`
- `ListUsers(ctx, ListUsersParams{...})` → `([]User, error)`
- `DeactivateUser(ctx, id)` → `error`

**Tabla Users:**
```
id              SERIAL PRIMARY KEY
email           VARCHAR(255) UNIQUE NOT NULL
name            VARCHAR(255) NOT NULL
password_hash   VARCHAR(255) NOT NULL
stripe_customer_id VARCHAR(255)
stripe_payment_method_id VARCHAR(255)
has_active_payment_method BOOLEAN DEFAULT FALSE
created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
updated_at      TIMESTAMP DEFAULT NOW()
is_active       BOOLEAN DEFAULT TRUE
```

### 4. Handlers (`internal/handlers/auth.go`)

**AuthHandler:**
```go
type AuthHandler struct {
    queries    *db.Queries
    jwtManager *security.JWTManager
}

func (h *AuthHandler) Register(ctx, req) (*RegisterResponse, error)
func (h *AuthHandler) Login(ctx, req) (*LoginResponse, error)
```

**Lógica Register:**
1. Validar email/name/password no vacíos
2. Validar password ≥6 caracteres
3. Verificar que email no existe (GetUserByEmail)
4. Hash password con bcrypt (GenerateFromPassword)
5. INSERT usuario (CreateUser)
6. Retornar user_id + email + mensaje

**Lógica Login:**
1. Validar email/password no vacíos
2. SELECT usuario por email (GetUserByEmail)
3. Comparar password vs hash (CompareHashAndPassword)
4. Generar JWT token (GenerateToken)
5. Retornar token + user_id + email

### 5. Middleware (`internal/middleware/auth.go`)

**AuthInterceptor:**
- Valida JWT en todas las requests (excepto Register/Login)
- Extrae token de header: `authorization: Bearer <token>`
- Valida firma y expiración
- Agrega user_id y email al context

**Helper Functions:**
- `GetUserIDFromContext(ctx)` → (int32, bool)
- `GetEmailFromContext(ctx)` → (string, bool)

**Métodos Públicos (sin auth):**
- `/auth.v1.AuthService/Register`
- `/auth.v1.AuthService/Login`
- `/health.v1.HealthService/Ping`
- `/health.v1.HealthService/Check`

### 6. Dependencias (`go.mod`)

```
github.com/golang-jwt/jwt/v5       - JWT token generation
golang.org/x/crypto                - Bcrypt password hashing
github.com/jackc/pgx/v5            - PostgreSQL driver
github.com/improbable-eng/grpc-web - gRPC-Web sin Envoy
```

## 🔄 Flujo Completo

```
1. Cliente Flutter → POST /auth/Register (gRPC-Web)
   {email: "user@example.com", name: "User", password: "secret"}
   ↓
2. AuthHandler.Register()
   ├─ Validar
   ├─ Hashear password (bcrypt)
   └─ INSERT en PostgreSQL
   ↓
3. Servidor → RegisterResponse
   {userId: 1, email: "user@example.com", message: "..."}

4. Cliente Flutter → POST /auth/Login (gRPC-Web)
   {email: "user@example.com", password: "secret"}
   ↓
5. AuthHandler.Login()
   ├─ SELECT usuario
   ├─ Comparar password
   ├─ Generar JWT
   └─ Retornar token
   ↓
6. Servidor → LoginResponse
   {jwtToken: "eyJhbGc...", userId: 1, email: "user@example.com"}

7. Cliente almacena token y lo usa en futuras requests
   Header: authorization: Bearer eyJhbGc...
   ↓
8. AuthInterceptor valida token en cada RPC
   ├─ Verifica firma
   ├─ Verifica expiración
   └─ Agrega user_id al context
   ↓
9. Handlers acceden a user_id vía GetUserIDFromContext(ctx)
```

## 🔐 Seguridad Implementada

✅ **Bcrypt Password Hashing**
- Cost = 10 (default recomendado)
- Salt automático
- Nunca almacenar plaintext

✅ **JWT Tokens**
- Firma HMAC-SHA256
- Secret key desde env (JWT_SECRET)
- Expiración: 24 horas
- Algoritmo: HS256

✅ **Validación Token**
- Verificar firma
- Verificar no-expirado
- Extraer claims de forma segura

✅ **Métodos Públicos**
- Solo Register/Login accesibles sin auth
- Resto de servicios requieren JWT válido

✅ **Comparación Segura**
- No revelar si existe usuario ("invalid email or password")
- bcrypt.CompareHashAndPassword es timing-safe

## 📋 Pasos para Activar

1. **Generar código:**
   ```bash
   make proto
   make sqlc
   ```

2. **Descargar deps:**
   ```bash
   go mod download
   ```

3. **Ya está cableado en main.go** (no hay que descomentar nada):
   ```go
   queries := db.New(dbConn)
   authHandler := handlers.NewAuthHandler(queries, jwtManager)
   authpb.RegisterAuthServiceServer(grpcServer, authHandler)
   ```

4. **Compilar:**
   ```bash
   make build
   ```

5. **Ejecutar:**
   ```bash
   make docker-up
   make db-migrate
   make run
   ```

6. **Probar:**
   ```bash
   # Register
   grpcurl -plaintext -d '{
     "email": "test@example.com",
     "name": "Test User",
     "password": "password123"
   }' localhost:50051 auth.v1.AuthService/Register
   
   # Login
   grpcurl -plaintext -d '{
     "email": "test@example.com",
     "password": "password123"
   }' localhost:50051 auth.v1.AuthService/Login
   ```

## 📚 Documentación

- **`AUTH_IMPLEMENTATION.md`** - Detalles técnicos completos
- **`QUICK_START.md`** - Guía rápida paso a paso
- **`DATABASE.md`** - SQLC y queries
- **`SETUP.md`** - Setup inicial del proyecto

## 🚀 Próximas Características (Futuro)

1. **Refresh Tokens** - Token de 30 días para refrescar JWT
2. **Email Verification** - Validar email antes de activar cuenta
3. **Password Reset** - Recuperación de contraseña
4. **Session Tracking** - Blacklist de tokens revocados
5. **OAuth2** - Integración con Google/GitHub
6. **2FA** - Autenticación de dos factores

## 📊 Comparación: Nuestro Approach vs genx

| Aspecto | can-you-buy-me (MVP) | genx (Production) |
|---------|----------------------|-------------------|
| Auth | JWT simple | Keycloak OAuth2 |
| Password | Bcrypt local | Keycloak |
| Roles | Simple (no roles) | RBAC complejo |
| Sessions | Stateless JWT | Stateful + cache |
| Tokens | 24h | Access + Refresh |
| Scope | Registro + Login | Empresa completa |

## 🎯 Implementado Correctamente

✅ Registro con validación y bcrypt
✅ Login con JWT generation
✅ Token validation en middleware
✅ Error handling seguro (no revelar info)
✅ Context extraction para otros servicios
✅ Configuración desde env variables
✅ Integración con PostgreSQL (sqlc)
✅ gRPC-Web compatible (sin Envoy)

## Lista de Archivos

**Código:**
- `proto/v1/auth.proto` - Definición del servicio
- `internal/security/jwt.go` - Generación y validación de JWT
- `internal/handlers/auth.go` - Implementación del servicio
- `internal/middleware/auth.go` - Interceptor de validación
- `db/queries/users.sql` - Queries para usuarios
- `cmd/server/main.go` - Registro del servicio (activo)

**Documentación:**
- `AUTH_IMPLEMENTATION.md` - Detalles técnicos
- `AUTH_SUMMARY.md` - Este archivo
- `QUICK_START.md` - Guía rápida
- `DATABASE.md` - SQLC
- `SETUP.md` - Setup inicial

**Config:**
- `go.mod` - Dependencias actualizadas
- `.env.example` - Variables de entorno
- `sqlc.yaml` - Configuración SQLC
- `Makefile` - Comandos de build

## Está Listo para:

- ✅ Registrar usuarios
- ✅ Autenticar con email/password
- ✅ Generar JWT tokens
- ✅ Validar tokens en otros servicios
- ✅ Extraer información del usuario desde context
- ✅ Proteger endpoints que requieren autenticación

**Siguiente paso:** Implementar AuctionService para listar y obtener subastas.
