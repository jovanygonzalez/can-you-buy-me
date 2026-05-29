# Setup Inicial - Can You Buy Me

Guía paso a paso para configurar el proyecto localmente por primera vez.

## Prerequisitos

Instalar primero:

1. **Go 1.22+** - https://golang.org/dl/
   - Verificar: `go version`

2. **Docker Desktop** - https://www.docker.com/products/docker-desktop
   - Verificar: `docker --version`

3. **Protocol Buffer Compiler (protoc)** - https://github.com/protocolbuffers/protobuf/releases
   - Descargar `protoc-23.0-win64.zip` (o más reciente)
   - Agregar `bin/` al PATH
   - Verificar: `protoc --version`

4. **SQLC** - SQL code generator
   - Instalar: `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`
   - Verificar: `sqlc version`

## Paso 1: Clonar/Descargar el Proyecto

```bash
cd C:\dev\can-you-buy-me
```

## Paso 2: Instalar Herramientas de Go

### Herramientas Protobuf
```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

### SQLC (SQL Code Generator)
```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```

Verificar:
```bash
sqlc version
```

## Paso 3: Descargar Dependencias Go

```bash
go mod download
go mod tidy
```

## Paso 4: Generar Código desde Protos y Queries SQL

### Generar Protos
```bash
make proto
# O si no tienes Make:
# protoc --go_out=. --go-grpc_out=. proto/v1/health.proto
# protoc --go_out=. --go-grpc_out=. proto/v1/auth.proto
# protoc --go_out=. --go-grpc_out=. proto/v1/auction.proto
# protoc --go_out=. --go-grpc_out=. proto/v1/payment.proto
```

Esto genera código en `pkg/gen/*/`.

### Generar SQLC
```bash
make sqlc
# O manualmente:
# sqlc generate
```

Esto genera código en `db/sqlc/`.

> **Fuente única de schema:** `sqlc` lee el schema de `db/migrations/00001_initial_schema.sql`
> (configurado en `sqlc.yaml`) y las queries de `db/queries/*.sql`. Es el
> **mismo** archivo que aplica `make db-up` a PostgreSQL, así que el
> código generado y la BD real nunca divergen.

**Nota:** Ambos pasos se ejecutan automáticamente con `make build`.

## Paso 5: Configurar Variables de Entorno

```bash
# Copiar el archivo de ejemplo
cp .env.example .env

# Editar .env si es necesario (para desarrollo, los defaults están bien)
# cat .env
```

> **Stripe (opcional pero recomendado):** Si configuras `STRIPE_API_KEY`
> (`sk_test_...`), el servidor verifica la conexión al arrancar y habilita
> `PaymentService`. Sin ella, el servidor arranca igual pero el servicio de
> pagos queda deshabilitado. Para webhooks locales, ver `STRIPE_API_KEY` +
> `STRIPE_WEBHOOK_SECRET` (obtenida con `stripe listen`, ver `.env.example`).

## Paso 6: Iniciar Servicios Docker

```bash
make docker-up

# Esperar ~10 segundos a que los contenedores estén listos
# Verificar:
docker ps
```

Debería mostrar 3 contenedores:
- `auction_postgres` (puerto 5435)
- `auction_redis` (puerto 6379)
- `auction_nats` (puerto 4222)

## Paso 7: Crear el Schema de Base de Datos

```bash
make db-up

# O manualmente:
# docker exec -it auction_postgres psql -U root -d auction_db -f db/migrations/00001_initial_schema.sql
```

Verificar:
```bash
docker exec -it auction_postgres psql -U root -d auction_db -c "\dt"
# Debería listar las tablas: users, auctions, bids, audit_log, payments
```

## Paso 8: (Opcional) Cargar Datos de Ejemplo

```bash
make db-seed

# O manualmente:
# docker exec -it auction_postgres psql -U root -d auction_db -f db/seeds/00001_seed_data.sql
```

Verificar:
```bash
docker exec -it auction_postgres psql -U root -d auction_db -c "SELECT id, title, base_price FROM auctions;"
```

## Paso 9: Compilar el Servidor

```bash
make build

# Verificar que se generó: bin/server
ls -la bin/
```

## Paso 10: Ejecutar el Servidor

```bash
make run

# O directamente:
# go run ./cmd/server/main.go
```

Deberías ver:
```
2024/XX/XX 12:XX:XX INFO Starting Can You Buy Me API Server...
2024/XX/XX 12:XX:XX INFO Server configuration grpc_port=50051 http_port=8070
2024/XX/XX 12:XX:XX INFO gRPC server listening port=50051
2024/XX/XX 12:XX:XX INFO HTTP/gRPC-Web server listening port=8070
2024/XX/XX 12:XX:XX INFO Health check successful
```

## Paso 11: Probar el Servidor

En otra terminal:

### Health Check HTTP
```bash
curl http://localhost:8070/health
# Debe retornar: {"status":"ok"}
```

### Usando grpcurl (instalar si no tienes)
```bash
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

# Listar servicios
grpcurl -plaintext localhost:50051 list

# Probar Ping
grpcurl -plaintext -d '{}' localhost:50051 health.v1.HealthService/Ping
# Debe retornar: { "message": "pong" }
```

## Paso 12: Detener los Servicios

```bash
# Parar el servidor: Ctrl+C en la terminal donde corre

# Parar Docker
make docker-down

# O manualmente:
# docker-compose -f containers/docker-compose.yml down
```

## Verificación Completa

Checklist final:

- [ ] Go 1.22+ instalado y en PATH
- [ ] Docker instalado y running
- [ ] protoc instalado y en PATH
- [ ] Dependencias Go descargadas (`go mod download`)
- [ ] Protos compilados (`make proto`)
- [ ] `.env` configurado
- [ ] Docker containers corriendo (`docker ps`)
- [ ] Base de datos schema creado (`psql ... \dt`)
- [ ] Servidor compilado (`make build`)
- [ ] Servidor ejecutado (`make run`)
- [ ] Health check responde (`curl http://localhost:8070/health`)

## Troubleshooting

### Error: "go: no Go files in /C/dev/can-you-buy-me"
- ✓ Ya tenemos archivos .go en cmd/server y internal/

### Error: "protoc: command not found"
```bash
# Windows: Descargar de https://github.com/protocolbuffers/protobuf/releases
# Extraer y agregar el bin/ al PATH del sistema

# Verificar:
protoc --version
```

### Error: "Failed to connect to database"
```bash
# Docker no está corriendo
docker ps  # Debería listar 3 contenedores

# Si no aparecen:
make docker-up
docker logs auction_postgres  # Ver logs
```

### Error: "Address already in use :50051"
```bash
# Otro proceso usa el puerto
# Cambiar en .env:
# GRPC_PORT=50052
# HTTP_PORT=8081
```

### Error: "connection refused" al hacer curl
```bash
# El servidor no está corriendo
# Terminal 1: make run
# Terminal 2: curl http://localhost:8070/health
```

## Próximos Pasos

Una vez que el servidor está corriendo:

1. **Conectar a PostgreSQL:** Ver `internal/handlers/` para ejemplos
2. **Implementar AuthService:** Registro y Login
3. **Implementar AuctionService:** Listar y obtener subastas
4. **PlaceBid:** Publicar a NATS JetStream
5. **Flutter Frontend:** Conectarse al servidor gRPC-Web

Ver `SERVER_README.md` para más detalles.
