# Quick Start - AuthService

Guía rápida para arrancar el servidor con AuthService (**ya implementado y cableado**).

## En Orden

### 1. Instalar herramientas (si no lo hiciste)
```bash
# Protoc
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# SQLC
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

### 2. Descargar dependencias
```bash
go mod download
go mod tidy
```

### 3. Generar código
```bash
# Generar protos
make proto

# Generar sqlc
make sqlc

# Verificar que existendos archivos
ls pkg/gen/auth/v1/          # Debe tener auth.pb.go y auth_grpc.pb.go
ls db/sqlc/                  # Debe tener users.sql.go, models.go, db.go
```

### 4. AuthService ya está cableado en main.go

No tienes que editar nada. `cmd/server/main.go` ya registra el servicio:

```go
// Crear queries para base de datos (variable dbConn)
queries := db.New(dbConn)

// Registrar AuthService
authHandler := handlers.NewAuthHandler(queries, jwtManager)
authpb.RegisterAuthServiceServer(grpcServer, authHandler)
```

### 5. Compilar
```bash
make build

# Si hay errores, revisar que db/sqlc/ tiene archivos generados
ls db/sqlc/*.go
```

### 6. Ejecutar
```bash
# Terminal 1: Docker
make docker-up
make db-migrate

# Terminal 2: Servidor
make run

# Deberías ver:
# INFO Starting Can You Buy Me API Server...
# INFO Database connection established
# INFO gRPC server listening port=50051
# INFO HTTP/gRPC-Web server listening port=8080
```

### 7. Probar Register
```bash
grpcurl -plaintext \
  -d '{
    "email": "test@example.com",
    "name": "Test User",
    "password": "password123"
  }' \
  localhost:50051 auth.v1.AuthService/Register
```

Respuesta esperada:
```json
{
  "userId": 1,
  "email": "test@example.com",
  "message": "User registered successfully"
}
```

### 8. Probar Login
```bash
grpcurl -plaintext \
  -d '{
    "email": "test@example.com",
    "password": "password123"
  }' \
  localhost:50051 auth.v1.AuthService/Login
```

Respuesta esperada:
```json
{
  "jwtToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "userId": 1,
  "email": "test@example.com"
}
```

## Troubleshooting

### "db.New undefined"
Necesitas generar sqlc:
```bash
make sqlc
ls db/sqlc/db.go
```

### "auth.v1.AuthService/Register: code = Unavailable"
El servidor no compiló o no arrancó. Verifica que el registro sigue presente:
```bash
grep -n "RegisterAuthServiceServer" cmd/server/main.go
```

### "user with this email already exists"
Normal - ya registraste ese usuario. Usa otro email:
```bash
grpcurl -plaintext -d '{
  "email": "test2@example.com",
  "name": "User 2",
  "password": "password123"
}' localhost:50051 auth.v1.AuthService/Register
```

### "password must be at least 6 characters"
Password muy corto. Usa al menos 6 caracteres.

### Error compilando
```bash
# Limpia y regenera todo
rm -rf db/sqlc pkg/gen
make proto
make sqlc
make build
```

## Verificación Final

✅ `db/sqlc/db.go` existe
✅ `db/sqlc/users.sql.go` existe
✅ `pkg/gen/auth/v1/auth_grpc.pb.go` existe
✅ `cmd/server/main.go` tiene `RegisterAuthServiceServer`
✅ Servidor compila sin errores
✅ Servidor conecta a PostgreSQL
✅ Register funciona
✅ Login funciona

## Próximos Pasos

1. **AuctionService** - Implementar GetAuction, ListAuctions
2. **PlaceBid** - Integrar con NATS JetStream
3. **Validación de Pujas** - Comparar contra highest_bid
4. **WebSocket** - Broadcast de pujas en tiempo real

Ver `AUTH_IMPLEMENTATION.md` para detalles.
