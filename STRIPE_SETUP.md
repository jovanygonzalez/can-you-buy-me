# Stripe Setup Guide

Guía para configurar Stripe en el servidor de "Can You Buy Me".

## Paso 1: Obtener las Claves de Stripe

### 1.1 Crear Cuenta en Stripe

Ir a: https://dashboard.stripe.com/register

- Email: tu_email
- Nombre: tu_nombre
- País: México (MX)
- Tipo: Individuo o Empresa

### 1.2 Obtener API Keys

En Stripe Dashboard:
1. Ve a: **Developers → API keys**
2. Verifica que estés en **Test mode** (superior izquierda)
3. Copia las dos claves:

**Test Mode Keys:**
- **Secret key:** `sk_test_...` (Guarda esto seguro)
- **Publishable key:** `pk_test_...` (Pública, para frontend)

**Para Producción (después):**
- Ve a **Live mode** (cambiar en la esquina superior)
- Usa `sk_live_...` y `pk_live_...`

### 1.3 Guardar en .env

```bash
# Copiar archivo
cp .env.example .env

# Editar .env
nano .env

# Agregar tus claves
STRIPE_API_KEY=sk_test_51Abc123...
STRIPE_PUBLISHABLE_KEY=pk_test_51Abc123...
```

## Paso 2: Configuración en Go

### 2.1 Estructura

```
internal/payment/
  └── stripe.go   # Cliente de Stripe
```

### 2.2 Funcionalidades

**NewStripeClient()** - Crea cliente
```go
client, err := payment.NewStripeClient()
// Verifica que STRIPE_API_KEY está configurada
```

**VerifyConnection()** - Verifica acceso a Stripe
```go
account, err := client.VerifyConnection()
// Retorna Account info si éxito
// Error si API key es inválida o Stripe no accesible
```

**IsLiveMode()** - Detecta live vs test
```go
if client.IsLiveMode() {
  // Producción
} else {
  // Test
}
```

## Paso 3: Iniciar el Servidor

### 3.1 Con Stripe Configurado

```bash
# Terminal 1: Docker
make docker-up
make db-migrate

# Terminal 2: Servidor
make run
```

**Output esperado:**
```
INFO Starting Can You Buy Me API Server...
INFO Stripe connection verified
  mode: test
  account_id: acct_1Abc123...
  account_email: tu_email@example.com
  account_name: Tu Nombre
INFO Database connection established
INFO gRPC server listening port=50051
INFO HTTP/gRPC-Web server listening port=8070
```

### 3.2 Sin Stripe (STRIPE_API_KEY no configurada)

```
WARN Stripe not configured (STRIPE_API_KEY not set or invalid)
WARN Payment features will be disabled
```

El servidor sigue funcionando, pero:
- ❌ No puedes crear Setup Intents
- ❌ No puedes cobrar
- ✅ Puedes probar autenticación y otras features

## Paso 4: Probar Stripe

### 4.1 Con grpcurl (ya implementado)

El endpoint `InitializeStripePayment` requiere JWT. La request va **vacía**
(el `user_id` se toma del token, NO del body):

```bash
grpcurl -plaintext \
  -H "authorization: Bearer $TOKEN" \
  -d '{}' \
  localhost:50051 payment.v1.PaymentService/InitializeStripePayment
```

Respuesta:
```json
{
  "clientSecret": "seti_..._secret_...",
  "stripeCustomerId": "cus_...",
  "message": "Setup Intent created successfully"
}
```

### 4.2 En Stripe Dashboard

1. Ve a **Customers**
2. Busca el cliente que creaste
3. Verifica Payment Methods guardados

### 4.3 Tarjetas de Prueba

Usa estas tarjetas en test mode:

| Tarjeta | Número | CVC | Fecha |
|---------|--------|-----|-------|
| Éxito | 4242 4242 4242 4242 | Cualquiera | Futuro |
| Decline | 4000 0000 0000 0002 | Cualquiera | Futuro |
| 3D Secure | 4000 0025 0000 3155 | Cualquiera | Futuro |

Ejemplos de uso:
- **Nombre:** John Doe
- **Email:** john@example.com
- **CVC:** 123
- **Fecha:** 12/25

## Estructura del Cliente de Stripe

```go
type StripeClient struct {
  apiKey string
}

// Métodos:
NewStripeClient()           // Crear cliente
VerifyConnection()          // Verificar acceso
GetAccountInfo()            // Info de cuenta
IsLiveMode()                // ¿Es live o test?
GetMode()                   // Retorna "live" o "test"
```

## Flujo de Inicialización

```
1. main.go inicia
   ↓
2. payment.NewStripeClient()
   ├─ Lee STRIPE_API_KEY de .env
   ├─ Si no existe: warning, continúa sin pago
   └─ Si existe: configura stripe.Key
   ↓
3. client.VerifyConnection()
   ├─ Llama account.GetByID() en Stripe API
   ├─ Si error: server detiene (exit 1)
   └─ Si éxito: retorna Account info
   ↓
4. Mostrar info de cuenta
   ├─ Mode (test/live)
   ├─ Account ID
   ├─ Email
   └─ Business name
   ↓
5. Servidor continúa normalmente
```

## Seguridad

✅ **Nunca commitear .env con STRIPE_API_KEY**
```bash
# .gitignore ya tiene:
.env
.env.local
```

✅ **Test key vs Live key**
- Test: `sk_test_...` - Usa en desarrollo
- Live: `sk_live_...` - Usa solo en producción

✅ **Variables de Entorno**
- En desarrollo: `.env` local
- En producción: Variables de entorno del servidor/container

✅ **Manejo de Errores**
- Si API key inválida: Server detiene
- Si Stripe está down: Server detiene
- Si no configurado: Warning, continúa sin pago

## Modos: Test vs Live

### Test Mode

```
✅ Usar durante desarrollo
✅ Usar tarjetas de prueba
✅ No carga real
✅ Datos de prueba en Dashboard
✅ API key: sk_test_...
```

### Live Mode

```
⚠️ Solo en producción
⚠️ Cargos reales
⚠️ Datos reales de clientes
⚠️ PCI compliance requerido
⚠️ API key: sk_live_...
```

**IMPORTANTE:** Cambiar a live mode solo cuando:
1. Todo funcione en test mode
2. Tengas seguridad en producción
3. Entiendas PCI compliance
4. Haya legal review

## Troubleshooting

### Error: "STRIPE_API_KEY environment variable not set"

```bash
# .env no configurado o no tiene STRIPE_API_KEY
# Solución:
cp .env.example .env
nano .env
# Agregar tu STRIPE_API_KEY
```

### Error: "failed to verify Stripe connection"

```
Posibles causas:
1. API key inválida/expirada
2. Stripe API está down
3. Red no tiene acceso a api.stripe.com
4. Key es de live pero estás en test (o viceversa)

Soluciones:
1. Verifica la key en https://dashboard.stripe.com/apikeys
2. Prueba: curl https://api.stripe.com/v1/account (con auth básica)
3. Verifica firewall/proxy
4. Verifica que usas sk_test_ en desarrollo
```

### Error: "mode: test" pero quería "live"

```
Esto es correcto para desarrollo.
Solo usa sk_live_ en producción.
Cambiar:
1. En Stripe Dashboard: Live mode (esquina superior)
2. Copiar sk_live_... y pk_live_...
3. Actualizar .env en producción
4. Restart servidor
```

### "Payment features will be disabled"

```
Significa que STRIPE_API_KEY no está configurada.
Continúa sin pago:
- ✅ Auth funciona
- ✅ Subastas funcionan
- ❌ Registro de tarjeta no funciona
- ❌ Cobro no funciona

Es OK para desarrollo inicial.
Configura cuando necesites probar pago.
```

## Siguientes Pasos

1. ✅ Configurar API key en .env
2. ✅ Iniciar servidor y ver "Stripe connection verified"
3. ✅ `InitializeStripePayment` endpoint (implementado)
4. ✅ Webhook `/webhooks/stripe` recibe `setup_intent.succeeded` (Fase 1)
5. ⏳ Setup Intent UI en Flutter
6. ⏳ `ConfirmPaymentMethod` / actualizar `has_active_payment_method` (Fase 2)
7. ⏳ ChargeWinner endpoint (futuro)

Ver `STRIPE_INTEGRATION.md` para arquitectura completa.

## Referencias

- **Stripe Dashboard:** https://dashboard.stripe.com
- **API Keys:** https://dashboard.stripe.com/apikeys
- **Setup Intents:** https://stripe.com/docs/payments/setup-intents
- **Payment Methods:** https://stripe.com/docs/api/payment_methods
- **Test Cards:** https://stripe.com/docs/testing
- **Go SDK:** https://github.com/stripe/stripe-go
