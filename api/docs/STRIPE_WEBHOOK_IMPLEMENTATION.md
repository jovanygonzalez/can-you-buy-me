# Stripe Webhook Implementation — setup_intent.succeeded

Implementación del endpoint webhook para recibir confirmación de Stripe cuando un usuario guarda su tarjeta exitosamente.

## ✅ Completado

### Archivos Creados

**1. `internal/webhook/stripe.go`** (NUEVO)
- `StripeWebhookHandler` con soporte para:
  - Verificación de firma con `webhook.ConstructEvent()`
  - Despacho de eventos (`setup_intent.succeeded`, `setup_intent.setup_failed`)
  - Logging detallado de eventos

```go
type StripeWebhookHandler struct {
    webhookSecret string
}

func (h *StripeWebhookHandler) Handle(w http.ResponseWriter, r *http.Request)
// Verifica firma → parsea evento → loggea detalles → retorna 200 OK
```

### Archivos Modificados

**2. `cmd/server/main.go`** (ACTUALIZADO)
- Instancia `StripeWebhookHandler` después de inicializar `stripeClient`
- Registra ruta `/webhooks/stripe` en httpMux
- Loggea si webhook está habilitado o deshabilitado

```go
webhookHandler, err := webhook.NewStripeWebhookHandler()
if err != nil {
    slog.Warn("Stripe webhook disabled...")
} else {
    httpMux.HandleFunc("/webhooks/stripe", webhookHandler.Handle)
    slog.Info("Stripe webhook endpoint registered")
}
```

**3. `.env.example`** (ACTUALIZADO)
- Descomentada `STRIPE_WEBHOOK_SECRET`
- Instrucciones para obtenerla con Stripe CLI local

## 🏗️ Arquitectura

```
HTTP Server (puerto 8070)
├─ /health                  ← health check simple
├─ /webhooks/stripe         ← NUEVO: recibe eventos de Stripe
└─ /                        ← gRPC-Web (catch-all)
```

El mismo servidor HTTP que sirve gRPC-Web ahora también maneja webhooks. **No se necesita puerto adicional**.

## 🧪 Cómo Probar

### Prerequisitos
- Tener `STRIPE_WEBHOOK_SECRET` configurada en `.env`
- Stripe CLI instalada: https://stripe.com/docs/stripe-cli

### Paso 1: Obtener webhook secret local

```bash
# Terminal 1: Escuchar webhooks
stripe listen --forward-to localhost:8070/webhooks/stripe

# Output:
# Ready! Your webhook signing secret is whsec_test_4neq70d8bq3ej00gkc0jnwjf (^C to quit)
```

### Paso 2: Configurar .env y reiniciar servidor

```bash
# Copiar el whsec_test_... al .env
STRIPE_WEBHOOK_SECRET=whsec_test_4neq70d8bq3ej00gkc0jnwjf

# Reiniciar el servidor
make run
# INFO Stripe webhook endpoint registered path=/webhooks/stripe
```

### Paso 3: Disparar evento de prueba (en otra terminal)

```bash
stripe trigger setup_intent.succeeded
```

### Paso 4: Ver logs en el servidor

```
INFO Stripe webhook received event_type=setup_intent.succeeded event_id=evt_test_...
INFO setup_intent.succeeded intent_id=seti_test_... customer_id=cus_test_... payment_method=pm_test_...
```

## 📋 Flujo Completo (Setup → Webhook)

```
1. Flutter → InitializeStripePayment(JWT)
   ↓
   Backend crea Customer + SetupIntent
   ↓
2. Flutter recibe client_secret

3. Flutter → Stripe.js.confirmCardSetup(client_secret)
   ↓
   Usuario ingresa tarjeta en Stripe hosted form
   ↓
4. Stripe valida tarjeta

5. Stripe → POST /webhooks/stripe (setup_intent.succeeded)
   ↓
   Backend:
   ├─ Verifica firma
   ├─ Parsea evento
   ├─ Loggea detalles
   └─ Responde 200 OK

6. (MVP Fase 2: Actualizar has_active_payment_method en BD)

7. Flutter ← Usuario puede pujar
```

## 🔐 Seguridad

✅ **Verificación de firma obligatoria** — `webhook.ConstructEvent()` valida el header `Stripe-Signature`
✅ **Body crudo** — el handler lee el body sin parsear para mantener la firma válida
✅ **Siempre 200 OK** — Stripe reintenta si recibe error, así que respondemos OK salvo para errores de firma

## 🚀 Eventos Soportados

| Evento | Manejador | Acción |
|--------|-----------|--------|
| `setup_intent.succeeded` | ✅ Implementado | Loggea detalles de intent |
| `setup_intent.setup_failed` | ✅ Implementado | Loggea error de setup |
| Otros | ✅ Genérico | Se loggean pero sin procesar |

## 📊 Eventos a Loggear

### setup_intent.succeeded
```
intent_id       → seti_...
customer_id     → cus_...
status          → succeeded
payment_method  → pm_...
event_id        → evt_...
```

### setup_intent.setup_failed
```
intent_id       → seti_...
customer_id     → cus_...
status          → setup_failed
error_code      → code_invalid_card | etc.
error_message   → "Your card was declined"
event_id        → evt_...
```

## 🔗 Próximos Pasos (Fase 2)

Después de que el webhook esté funcionando:

1. **Crear la query `GetUserByStripeCustomerID`** (NO existe aún en `db/queries/users.sql`):
   ```sql
   -- name: GetUserByStripeCustomerID :one
   SELECT * FROM users WHERE stripe_customer_id = $1;
   ```
   Luego, en `handleSetupIntentSucceeded`:
   ```go
   user, err := queries.GetUserByStripeCustomerID(ctx, pgtype.Text{
       String: setupIntent.Customer.ID, Valid: true,
   })
   if err != nil {
       slog.Error("Customer not found", "customer_id", setupIntent.Customer.ID)
       return
   }
   ```

2. **Actualizar has_active_payment_method = true** (sqlc: struct de params, `pgtype.Bool`):
   ```go
   _, err = queries.UpdateUserPaymentMethodStatus(ctx, db.UpdateUserPaymentMethodStatusParams{
       HasActivePaymentMethod: pgtype.Bool{Bool: true, Valid: true},
       ID:                     user.ID,
   })
   ```

3. **Guardar payment_method_id en BD** (opcional, para futuro cobro):
   ```go
   _, err = queries.UpdateUserStripePaymentMethodWithStatus(ctx, db.UpdateUserStripePaymentMethodWithStatusParams{
       StripePaymentMethodID:  pgtype.Text{String: setupIntent.PaymentMethod.ID, Valid: true},
       HasActivePaymentMethod: pgtype.Bool{Bool: true, Valid: true},
       ID:                     user.ID,
   })
   ```

## 📝 Stripe Dashboard vs Stripe CLI

### Stripe CLI (Desarrollo Local)
```bash
stripe listen --forward-to localhost:8070/webhooks/stripe
# whsec_test_...  (válido solo localmente)
```

### Stripe Dashboard (Producción)
```
Dashboard → Webhooks → Crear endpoint
# https://api.ejemplo.com/webhooks/stripe
# whsec_live_... (válido en producción)
```

## ✨ Estructura del Código

```
internal/webhook/
└── stripe.go
    ├── StripeWebhookHandler
    ├── NewStripeWebhookHandler()
    ├── Handle()
    ├── handleSetupIntentSucceeded()
    └── handleSetupIntentFailed()
```

Métodos:
- **Handle**: Entrada del webhook, verifica firma, despacha evento
- **handleSetupIntentSucceeded**: Procesa success (loggea por ahora)
- **handleSetupIntentFailed**: Procesa error (loggea fallo)

## 🧬 Conversión de Evento Raw a SetupIntent

```go
var setupIntent stripe.SetupIntent
if err := json.Unmarshal(event.Data.Raw, &setupIntent); err != nil {
    // Error
}
// setupIntent ahora tiene: ID, Customer, Status, PaymentMethod, LastSetupError, etc.
```

> ⚠️ **`Customer`, `PaymentMethod` y `LastSetupError` son PUNTEROS** (`*Customer`,
> `*PaymentMethod`, `*Error`). Pueden venir `nil` en el payload — p.ej.
> `stripe trigger setup_intent.succeeded` suele NO adjuntar customer. Hay que
> chequear nil antes de acceder a `.ID`/`.Code`, o el handler hace panic (el
> server lo recupera pero NO responde 200 y Stripe reintenta). El código ya
> aplica estas guardas.

## 📍 Rutas Registradas

| Ruta | Puerto | Métodos | Manejador | Requisito |
|------|--------|---------|-----------|-----------|
| `/health` | 8070 | GET | HTTP simple | Siempre |
| `/webhooks/stripe` | 8070 | POST | StripeWebhookHandler | STRIPE_WEBHOOK_SECRET |
| `/` | 8070 | POST, [gRPC methods] | grpcWebHandler | Siempre (catch-all) |

## 🎯 MVP Fase 1 (Actual)

✅ Recibir evento setup_intent.succeeded
✅ Verificar firma
✅ Parsear evento
✅ Loggear detalles
✅ Responder 200 OK a Stripe

## ❌ MVP Fase 2 (Futuro)

- [ ] Conectar customer_id con usuario
- [ ] Actualizar has_active_payment_method = true
- [ ] Guardar payment_method_id
- [ ] Notificar al usuario
- [ ] Bloquear rebotes de Stripe

## Verificación Rápida

```bash
# 1. Compilar
make build

# 2. Ejecutar
make run
# Debe mostrar: INFO Stripe webhook endpoint registered

# 3. En otra terminal, escuchar webhooks
stripe listen --forward-to localhost:8070/webhooks/stripe

# 4. Disparar evento
stripe trigger setup_intent.succeeded

# 5. Los logs del servidor deben mostrar:
# INFO Stripe webhook received event_type=setup_intent.succeeded
# INFO setup_intent.succeeded intent_id=seti_...
```

## Estado

✅ **COMPLETADO Y FUNCIONAL**

El webhook está listo para recibir eventos de Stripe. La siguiente fase es conectar esos eventos con los usuarios en la BD.
