# InitializeStripePayment Implementation - COMPLETE

Implementación completada del endpoint `InitializeStripePayment` para crear Stripe Customers y Setup Intents.

## ✅ Archivos Creados

### 1. `proto/v1/payment.proto` (NUEVO)
```protobuf
service PaymentService {
  rpc InitializeStripePayment(InitializeStripePaymentRequest) returns (InitializeStripePaymentResponse);
}
```
- Request: vacía (user_id viene del JWT context)
- Response: client_secret, stripe_customer_id, message

### 2. `internal/handlers/payment.go` (NUEVO)
- `PaymentHandler` struct
- Implementa `PaymentServiceServer`
- Orquesta: GetUser → CreateCustomer (si no existe) → CreateSetupIntent
- Usa `middleware.GetUserIDFromContext()` para extraer JWT
- Manejo de errores con códigos gRPC apropiados

## ✅ Archivos Modificados

### 3. `internal/payment/stripe.go` (ACTUALIZADO)
Agregados dos nuevos métodos:
- `CreateCustomer(email, name string) (*stripe.Customer, error)` 
  - Usa `stripe-go/v76/customer`
- `CreateSetupIntent(stripeCustomerID string) (*stripe.SetupIntent, error)`
  - Usa `stripe-go/v76/setupintent`
  - Payment method types: `["card"]`
  - Usage: `off_session` (permite cobrar sin que usuario esté presente)

### 4. `internal/grpc/server.go` (ACTUALIZADO)
- `NewGRPCServer` ahora acepta `extraOpts ...grpc.ServerOption`
- Permite pasar interceptores y otras opciones al crear el servidor

### 5. `cmd/server/main.go` (ACTUALIZADO - 5 cambios)
a) **Almacenar `stripeClient`** como variable persistente
b) **Cablear `AuthInterceptor`** al crear el gRPC server:
   ```go
   grpc.ChainUnaryInterceptor(middleware.AuthInterceptor(jwtManager))
   ```
c) **Activar `AuthHandler`** (descomentar)
d) **Registrar `PaymentHandler`** (con validación de stripeClient nil)
e) **Renombrar variable `db` → `dbConn`** para evitar conflicto con paquete sqlc

### 6. `Makefile` (ACTUALIZADO)
Agregado `proto/v1/payment.proto` al comando `make proto`

## 🚀 Pasos Para Ejecutar

### Paso 1: Generar código desde protos
```bash
go mod tidy
make proto      # Genera pkg/gen/payment/v1/
make sqlc       # Regenera (no hay cambios, pero asegura que sqlc está listo)
```

### Paso 2: Compilar
```bash
make build      # No debe haber errores
```

### Paso 3: Ejecutar servidor
```bash
# Terminal 1: Services
make docker-up
make db-migrate

# Terminal 2: Server
make run
```

**Output esperado:**
```
INFO Starting Can You Buy Me API Server...
INFO Stripe connection verified mode=test ...
INFO Database connection established ...
INFO Auth service registered
INFO Payment service registered
INFO gRPC server listening port=50051
INFO HTTP/gRPC-Web server listening port=8080
```

## 🧪 Pasos Para Probar

### 1. Registrar usuario
```bash
grpcurl -plaintext \
  -d '{"email":"test@example.com","name":"Test User","password":"password123"}' \
  localhost:50051 auth.v1.AuthService/Register
```

Respuesta:
```json
{
  "userId": 1,
  "email": "test@example.com",
  "message": "User registered successfully"
}
```

### 2. Login para obtener JWT
```bash
TOKEN=$(grpcurl -plaintext \
  -d '{"email":"test@example.com","password":"password123"}' \
  localhost:50051 auth.v1.AuthService/Login | jq -r '.jwtToken')

echo $TOKEN  # Verificar que tiene valor
```

### 3. Llamar InitializeStripePayment CON JWT
```bash
grpcurl -plaintext \
  -H "authorization: Bearer $TOKEN" \
  -d '{}' \
  localhost:50051 payment.v1.PaymentService/InitializeStripePayment
```

Respuesta esperada:
```json
{
  "clientSecret": "seti_1Abc123_secret_xyz",
  "stripeCustomerId": "cus_Abc123",
  "message": "Setup Intent created successfully"
}
```

### 4. Llamar SIN JWT (debe fallar)
```bash
grpcurl -plaintext \
  -d '{}' \
  localhost:50051 payment.v1.PaymentService/InitializeStripePayment
```

Error esperado:
```
Code: Unauthenticated
Message: user_id not found in context
```

### 5. Probar idempotencia
Llamar 2 veces el mismo endpoint con el mismo JWT:
```bash
# Primera llamada
grpcurl -plaintext \
  -H "authorization: Bearer $TOKEN" \
  -d '{}' \
  localhost:50051 payment.v1.PaymentService/InitializeStripePayment

# Segunda llamada (mismo usuario)
grpcurl -plaintext \
  -H "authorization: Bearer $TOKEN" \
  -d '{}' \
  localhost:50051 payment.v1.PaymentService/InitializeStripePayment
```

**Comportamiento esperado:**
- Primera llamada: crea Stripe Customer y Setup Intent
- Segunda llamada: reutiliza Customer (mismo ID), crea nuevo Setup Intent

### 6. Verificar en Stripe Dashboard
1. Ve a **Customers** → busca `test@example.com`
2. Verifica que aparece el cliente
3. Ve a **Setup Intents** → verifica que aparecen 2 intents (de la idempotencia test)

## 🔍 Validaciones Implementadas

✅ **JWT requerido** — AuthInterceptor rechaza requests sin token válido
✅ **User extraído del context** — `middleware.GetUserIDFromContext()`
✅ **Idempotencia de Customer** — reutiliza si ya existe en DB
✅ **Setup Intent con Usage=off_session** — permite cobros futuros sin usuario presente
✅ **Stripe opcional** — si no configurado, servicio no se registra

## 🔄 Flujo Completo

```
1. Flutter → Login → obtiene JWT
2. Flutter → InitializeStripePayment(JWT)
3. Server:
   ├─ AuthInterceptor valida JWT
   ├─ PaymentHandler.InitializeStripePayment()
   ├─ GetUserByID(user_id del JWT)
   ├─ Si stripe_customer_id vacío:
   │  ├─ CreateCustomer(email, name)
   │  └─ UpdateUserStripeCustomer()
   ├─ CreateSetupIntent(stripe_customer_id)
   └─ Retorna client_secret
4. Flutter → Stripe.js.confirmCardSetup(client_secret)
5. Stripe valida y guarda la tarjeta
6. Flutter → ConfirmPaymentMethod(payment_method_id) [futuro endpoint]
```

## 📚 Documentación Relacionada

- **`STRIPE_SETUP.md`** - Setup de Stripe API keys
- **`STRIPE_INTEGRATION.md`** - Arquitectura general de pagos
- **`STRIPE_CONFIG_SUMMARY.md`** - Resumen de configuración
- **`AUTH_SUMMARY.md`** - Autenticación JWT

## 🚨 Si hay errores

### "unimplemented" en InitializeStripePayment
- ✅ Ejecutaste `make proto`?
- ✅ Ejecutaste `make build`?
- ✅ Stripe está configurado en .env?
- ✅ Servidor ve "Payment service registered"?

### "user_id not found in context"
- ✅ ¿Incluiste el header `authorization: Bearer <token>`?
- ✅ ¿Es un JWT válido de login exitoso?
- ✅ El middleware.AuthInterceptor se cableó en main.go?

### "failed to create Stripe customer"
- ✅ ¿STRIPE_API_KEY está en .env?
- ✅ ¿Es una clave de Stripe válida?
- ✅ ¿El servidor mostró "Stripe connection verified"?

## ✨ Próximos Pasos

### Inmediato (Próxima fase)
1. **ConfirmPaymentMethod endpoint** — guardar payment_method_id en DB
2. **Flutter WebUI** — integrar Stripe.js y confirmCardSetup()

### Futuro
1. **ChargeWinner endpoint** — cobrar automáticamente
2. **Webhook handling** — escuchar eventos de Stripe
3. **Refunds** — procesar devoluciones

## Resumen de Cambios

| Aspecto | Antes | Después |
|---------|-------|---------|
| Handlers registrados | 1 (Health) | 3 (Health + Auth + Payment) |
| AuthInterceptor cableado | ❌ No | ✅ Sí |
| Stripe Customer creation | ❌ No | ✅ Sí |
| Setup Intent creation | ❌ No | ✅ Sí |
| Proto services | 2 | 4 (+ payment) |
| JWT enforcement | ⚠️ Definido pero no usado | ✅ Activo en todas las requests |

## Status

✅ **COMPLETADO Y LISTO PARA TESTEAR**

Todos los componentes están implementados:
- Proto file definido
- Handler implementado
- Stripe client extendido
- Main.go cableado
- Makefile actualizado
- Documentación completa

**Siguiente:** Ejecutar `make proto && make build && make run` y probar con grpcurl
