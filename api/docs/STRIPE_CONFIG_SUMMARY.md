# Stripe Configuration Summary

Implementación de Stripe en "Can You Buy Me" completada.

## ✅ Completado

### 1. Librería de Stripe
- **Versión:** `github.com/stripe/stripe-go/v76` (última estable)
- **Agregado a:** `go.mod`
- **Paquete:** `internal/payment/stripe.go`

### 2. Cliente de Stripe

```go
type StripeClient struct {
  apiKey string
}

// Métodos:
- NewStripeClient()      // Crear cliente, leer STRIPE_API_KEY de env
- VerifyConnection()     // Conectar a Stripe API, validar credenciales
- GetAccountInfo()       // Obtener info de la cuenta
- IsLiveMode()          // Detectar sk_test_ vs sk_live_
- GetMode()             // Retorna "test" o "live"
```

### 3. Verificación en main.go

Al iniciar el servidor:

```
1. Leer STRIPE_API_KEY de .env
   ├─ Si no existe: WARN (pago deshabilitado, continúa)
   └─ Si existe: configura cliente
   
2. Llamar client.VerifyConnection()
   ├─ Si error: ERROR (exit 1, falla el servidor)
   └─ Si éxito: muestra info de cuenta
   
3. Output:
   INFO Stripe connection verified
     mode: test
     account_id: acct_...
     account_email: user@example.com
     account_name: My Business Name
```

### 4. Variables de Entorno

```bash
# .env
STRIPE_API_KEY=sk_test_...              # Secret key
STRIPE_PUBLISHABLE_KEY=pk_test_...      # Publishable key (para frontend)
```

**Modos:**
- **Test:** `sk_test_...` (desarrollo)
- **Live:** `sk_live_...` (producción)

### 5. Validación de Credenciales

El servidor **rechaza iniciar** si:
- ✅ STRIPE_API_KEY está configurada pero **inválida**
- ✅ No hay acceso a Stripe API
- ✅ Key ha expirado

El servidor **continúa sin pago** si:
- ⚠️ STRIPE_API_KEY no está configurada

## 🔄 Flujo de Inicialización

```
make run
   ↓
main.go
   ├─ Cargar .env (godotenv)
   ├─ Conectar a PostgreSQL
   │
   ├─→ NewStripeClient()
   │   └─ Leer STRIPE_API_KEY
   │
   ├─→ VerifyConnection()
   │   ├─ Llamar account.Get() a Stripe API
   │   ├─ Si error: log error y EXIT
   │   └─ Si éxito: retorna Account
   │
   ├─ Log info de cuenta
   │   ├─ Mode (test/live)
   │   ├─ Account ID
   │   ├─ Email
   │   └─ Business name
   │
   ├─ Iniciar gRPC + HTTP servers
   └─ Ready para requests
```

## 📋 Archivos Modificados

| Archivo | Cambio |
|---------|--------|
| `go.mod` | Agregado `github.com/stripe/stripe-go/v76` |
| `cmd/server/main.go` | Verificación de Stripe al iniciar |
| `.env.example` | Variables de Stripe con comentarios |
| `internal/payment/stripe.go` | NUEVO: Cliente de Stripe |
| `STRIPE_SETUP.md` | NUEVO: Guía de setup |

## 🚀 Para Usar

### 1. Obtener claves de Stripe

```bash
# Ir a https://dashboard.stripe.com/apikeys
# Copiar sk_test_... (en Test Mode)
```

### 2. Configurar .env

```bash
cp .env.example .env
nano .env

# Agregar:
STRIPE_API_KEY=sk_test_51Abc123...
```

### 3. Iniciar servidor

```bash
make run

# Output esperado:
# INFO Stripe connection verified
#   mode: test
#   account_id: acct_1Abc123...
```

## 🔐 Seguridad

✅ API key solo en .env (no en código)
✅ .gitignore contiene .env
✅ Detecta test vs live automáticamente
✅ Falla rápido si credenciales inválidas
✅ Nunca expone API key en logs

## 🧪 Probar

### Sin Stripe (desarrollo inicial)

```bash
# No configurar STRIPE_API_KEY
make run

# Output:
# WARN Stripe not configured
# WARN Payment features will be disabled
# ✓ Servidor funciona igual (sin pago)
```

### Con Stripe (test mode)

```bash
# Configurar STRIPE_API_KEY=sk_test_...
make run

# Output:
# INFO Stripe connection verified
# mode: test
# ✓ Servidor listo para crear customers, setup intents, etc.
```

### Con Stripe (live mode)

```bash
# Usar STRIPE_API_KEY=sk_live_...
# (Solo en producción)
make run

# Output:
# INFO Stripe connection verified
# mode: live
# ✓ Cargos reales funcionan
```

## 📚 Documentación Relacionada

- **`STRIPE_SETUP.md`** - Guía paso a paso
- **`STRIPE_INTEGRATION.md`** - Arquitectura y flujos
- **`USERS_TABLE_CHANGES.md`** - Campo `has_active_payment_method`

## 🔗 Métodos Disponibles

```go
// Crear cliente (automático en main.go)
client, err := payment.NewStripeClient()

// Verificar conexión (automático en main.go)
account, err := client.VerifyConnection()

// Info de cuenta
account, err := client.GetAccountInfo()

// Detectar modo
isLive := client.IsLiveMode()
mode := client.GetMode()  // "test" o "live"
```

## Estado de Implementaciones

1. ✅ **InitializeStripePayment endpoint** — IMPLEMENTADO
   - Crea Stripe Customer (idempotente) + Setup Intent, retorna client_secret
   - Ver `internal/handlers/payment.go`

2. ✅ **Webhook handling** — IMPLEMENTADO (Fase 1)
   - Verifica firma `Stripe-Signature`, recibe `setup_intent.succeeded`
   - Ver `internal/webhook/stripe.go`. Loggea; aún NO actualiza la BD (Fase 2).

3. ⏳ **ConfirmPaymentMethod endpoint** — pendiente
   - Actualizar `has_active_payment_method = true` (lo hará el webhook en Fase 2)

4. ⏳ **ChargeWinner endpoint** — pendiente
   - Cobrar al ganador post-subasta y guardar en tabla payments

## 🎯 Estado Actual

✅ Cliente de Stripe creado y listo
✅ Verificación de credenciales en startup
✅ Detección de modo (test/live)
✅ Manejo de errores apropiado
✅ Documentación completa
✅ Variables de entorno configuradas

**Listo para:** Implementar endpoints de InitializeStripePayment y ConfirmPaymentMethod

## Versión de Librería

```
Stripe Go SDK: v76.0.0 (última estable)
Compatibilidad: Go 1.22+
API version: 2024-04-10 (última)
```

### ¿Por qué v76?

- ✅ Última versión estable (2024)
- ✅ Soporte a largo plazo
- ✅ Todos los features principales
- ✅ Mejor documentation
- ✅ Security patches activos

Para futuro: actualizar a v77+ cuando esté disponible
