# Stripe Integration - MVP

Documentación de la integración de Stripe para el MVP de "Can You Buy Me".

## Arquitectura Stripe en MVP

```
Flutter Web User
    ↓
1. Register / Login
    ↓
2. (Opcional) Setup Intent
    ↓ (User agrrega tarjeta)
Stripe Hosted Page
    ↓
3. Validar tarjeta
    ↓
4. Guardar Payment Method ID
    ↓
PostgreSQL
    stripe_customer_id
    stripe_payment_method_id
    has_active_payment_method ← booleano para verificación rápida
```

## Filosofía MVP: Manual Hybrid

**No automatizar cobros durante pujas.** Razones:
1. Reduce scope de desarrollo
2. Evita bugs de retención/autorizaciones complejas
3. Permite validar modelo antes de automatizar
4. Admin controla qué cobrar y cuándo

**Flujo:**
```
Usuario puja (sin cargo)
    ↓
Admin cierra subasta manualmente
    ↓
Admin verifica tarjeta guardada (has_active_payment_method = true)
    ↓
Admin cobra manualmente desde Stripe Dashboard
    ↓
Usuario notificado (email manual por ahora)
```

## Tabla de Usuarios Actualizada

```sql
CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  
  -- Autenticación (AuthService)
  email VARCHAR(255) NOT NULL UNIQUE,
  name VARCHAR(255) NOT NULL,
  password_hash VARCHAR(255) NOT NULL,
  
  -- Stripe Setup Intents
  stripe_customer_id VARCHAR(255),              -- ID creado en primer Setup Intent
  stripe_payment_method_id VARCHAR(255),         -- ID del método guardado
  has_active_payment_method BOOLEAN DEFAULT FALSE,  -- ← NUEVO: Flag para verificación rápida
  
  -- Metadata
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  is_active BOOLEAN DEFAULT TRUE
);

CREATE INDEX idx_users_has_active_payment ON users(has_active_payment_method);
```

## Cambios en DB/Queries

### Nuevos Queries

**`UpdateUserPaymentMethodStatus`**
```sql
UPDATE users
SET has_active_payment_method = $1, updated_at = CURRENT_TIMESTAMP
WHERE id = $2
RETURNING *;
```
Uso: Cuando verificas con Stripe si hay método válido.

**`UpdateUserStripePaymentMethodWithStatus`**
```sql
UPDATE users
SET
  stripe_payment_method_id = $1,
  has_active_payment_method = $2,
  updated_at = CURRENT_TIMESTAMP
WHERE id = $3
RETURNING *;
```
Uso: Después de crear Setup Intent, guarda ambos valores.

**`ListUsersWithActivePayment`**
```sql
SELECT * FROM users
WHERE is_active = true AND has_active_payment_method = true
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;
```
Uso: Listar usuarios que pueden pujar.

## Flujo: Registro + Setup Intent

### 1. Usuario se registra
```protobuf
// auth.proto - Ya implementado
rpc Register(RegisterRequest) returns (RegisterResponse);
```

Backend:
- Valida email/name/password
- Hashea password
- INSERT en users (stripe_customer_id = NULL)

### 2. (Después de registrar) Crear Stripe Customer

**Endpoint — YA IMPLEMENTADO** (`proto/v1/payment.proto`, `internal/handlers/payment.go`):
```protobuf
rpc InitializeStripePayment(InitializeStripePaymentRequest) returns (InitializeStripePaymentResponse);

message InitializeStripePaymentRequest {
  // vacía — el user_id viene del JWT (context), NO del body
}

message InitializeStripePaymentResponse {
  string client_secret      = 1;  // Para SetupIntent en Flutter/Stripe.js
  string stripe_customer_id = 2;
  string message            = 3;
}
```

Backend (API real de stripe-go/v76 — paquetes `customer`, `setupintent`):
```go
// 1. Crear Stripe Customer
cust, err := customer.New(&stripe.CustomerParams{
  Email: stripe.String(user.Email),
  Name:  stripe.String(user.Name),
})

// 2. Guardar en DB (sqlc: struct de params, customer_id es pgtype.Text)
queries.UpdateUserStripeCustomer(ctx, db.UpdateUserStripeCustomerParams{
  StripeCustomerID: pgtype.Text{String: cust.ID, Valid: true},
  ID:               userID,
})

// 3. Crear Setup Intent
si, err := setupintent.New(&stripe.SetupIntentParams{
  Customer:           stripe.String(cust.ID),
  PaymentMethodTypes: []*string{stripe.String("card")},
  Usage:              stripe.String(string(stripe.SetupIntentUsageOffSession)),
})

// 4. Retornar client_secret al frontend
return &pb.InitializeStripePaymentResponse{
  ClientSecret:     si.ClientSecret,
  StripeCustomerId: cust.ID,
}
```

### 3. Flutter Web: Cargar Tarjeta

```dart
// Pseudo-código Flutter

// 1. Obtener client_secret del backend
final response = await channel.InitializeStripePayment(InitializePaymentRequest(...));

// 2. Confirmar Setup Intent con Stripe.js
StripeJS.confirmCardSetup(response.clientSecret, {
  payment_method: {
    card: cardElement,
    billing_details: { name: userName },
  },
});

// 3. Si success: notificar al backend que se guardó
channel.ConfirmPaymentMethod(ConfirmPaymentMethodRequest(
  userID: userId,
  stripePaymentMethodID: confirmResult.setupIntent.payment_method,
));
```

### 4. Backend: Confirmar y Guardar

**Endpoint a implementar:**
```protobuf
// payment.proto - FUTURO
rpc ConfirmPaymentMethod(ConfirmPaymentMethodRequest) returns (ConfirmPaymentMethodResponse);

message ConfirmPaymentMethodRequest {
  int32 user_id = 1;
  string stripe_payment_method_id = 2;
}

message ConfirmPaymentMethodResponse {
  bool success = 1;
  string message = 2;
}
```

Backend:
```go
// 1. Verificar que el payment_method pertenece al customer
stripe.PaymentMethod, err := client.PaymentMethods.Get(payment_method_id, nil)
if stripe.PaymentMethod.Customer.ID != user.StripeCustomerID {
  return error("Payment method doesn't belong to this user")
}

// 2. Guardar en DB
queries.UpdateUserStripePaymentMethodWithStatus(
  ctx,
  payment_method_id,
  true,  // has_active_payment_method = true
  user_id,
)

// 3. Retornar OK
return &ConfirmPaymentMethodResponse{
  Success: true,
  Message: "Payment method saved successfully",
}
```

## Flujo: Cobro Manual Post-Subasta

### 1. Admin ve que subasta terminó
```
Dashboard (futuro):
- Subasta cerrada
- Ganador: User ID 5
- Precio final: $100 MXN
- Estado: Pendiente cobro
```

### 2. Admin verifica si usuario tiene tarjeta guardada

```sql
SELECT has_active_payment_method, stripe_customer_id, stripe_payment_method_id
FROM users
WHERE id = 5;

-- Resultado: true, "cus_...", "pm_..."
```

### 3. Admin cobra desde Stripe Dashboard

**Opción 1: Dashboard Stripe (Manual simple)**
- Ir a Customers → cus_...
- Click "Create payment"
- Seleccionar payment method guardado
- Enter amount: $100
- Confirm

**Opción 2: Endpoint para auto-cobro (Futuro)**
```protobuf
rpc ChargeWinner(ChargeWinnerRequest) returns (ChargeWinnerResponse);

message ChargeWinnerRequest {
  int32 auction_id = 1;
}

message ChargeWinnerResponse {
  bool success = 1;
  string stripe_charge_id = 2;
  string message = 3;
}
```

Backend:
```go
// 1. Obtener subasta y ganador
auction, err := queries.GetAuctionByID(ctx, auction_id)
winner, err := queries.GetUserByID(ctx, auction.WinnerID)

// 2. Verificar que tiene tarjeta
if !winner.HasActivePaymentMethod {
  return error("Winner doesn't have active payment method")
}

// 3. Crear cargo en Stripe
charge, err := client.Charges.New(&stripe.ChargeParams{
  Amount:      100,  // centavos
  Currency:    "mxn",
  Customer:    winner.StripeCustomerID,
  PaymentMethod: winner.StripePaymentMethodID,
})

// 4. Guardar en payments table
queries.InsertPayment(ctx, {
  user_id: winner.ID,
  auction_id: auction.ID,
  stripe_charge_id: charge.ID,
  amount: 100.00,
  status: "succeeded",
})

// 5. Retornar OK
return &ChargeWinnerResponse{
  Success: true,
  StripeChargeID: charge.ID,
  Message: "Charge processed successfully",
}
```

## Campos en Users Table

| Campo | Tipo | Descripción |
|-------|------|-------------|
| id | SERIAL | Primary key |
| email | VARCHAR(255) | Email único |
| name | VARCHAR(255) | Nombre del usuario |
| password_hash | VARCHAR(255) | Bcrypt hash |
| stripe_customer_id | VARCHAR(255) | ID del cliente en Stripe |
| stripe_payment_method_id | VARCHAR(255) | ID del payment method guardado |
| has_active_payment_method | BOOLEAN | **NUEVO:** Flag para validación rápida sin consultar Stripe |
| created_at | TIMESTAMP | Fecha de creación |
| updated_at | TIMESTAMP | Última actualización |
| is_active | BOOLEAN | Si la cuenta está activa |

## Índices

```sql
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_stripe_customer_id ON users(stripe_customer_id);
CREATE INDEX idx_users_has_active_payment ON users(has_active_payment_method);  -- NUEVO
```

El índice en `has_active_payment_method` permite:
- Listar usuarios que pueden pujar rápidamente
- Filtrar en querys complejas sin table scan

## Tabla Payments (Para Auditoría)

```sql
CREATE TABLE payments (
  id BIGSERIAL PRIMARY KEY,
  
  user_id INTEGER NOT NULL REFERENCES users(id),
  auction_id INTEGER NOT NULL REFERENCES auctions(id),
  
  -- Stripe References
  stripe_charge_id VARCHAR(255) UNIQUE,
  stripe_payment_intent_id VARCHAR(255) UNIQUE,
  
  amount DECIMAL(12, 2) NOT NULL,
  currency VARCHAR(3) DEFAULT 'MXN',
  
  -- Estado
  status VARCHAR(50),  -- pending, succeeded, failed, refunded
  error_message TEXT,
  
  -- Flag para MVP
  is_manual BOOLEAN DEFAULT FALSE,  -- true si fue manual desde Dashboard
  
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## MVP Scope: Qué está incluido

✅ Almacenar `stripe_customer_id` y `stripe_payment_method_id`
✅ Flag `has_active_payment_method` para validación rápida
✅ Queries para actualizar ambos valores
✅ Índice para buscar usuarios con pago activo
✅ Tabla de auditoría para cobros

## MVP Scope: Qué es futuro

❌ Crear Stripe Customer automático (endpoint a implementar)
❌ Setup Intent UI (Flutter Web)
❌ Confirmación de payment method (endpoint a implementar)
❌ Cobro automático post-subasta (endpoint a implementar)
❌ Refunds
❌ Subscriptions

## Próximas Funcionalidades

### Fase 1 (MVP)
1. InitializeStripePayment - Crear customer + setup intent
2. ConfirmPaymentMethod - Guardar tarjeta
3. Manual charging desde Stripe Dashboard
4. Auditoría en tabla payments

### Fase 2 (Post-MVP)
1. Cobro automático post-subasta
2. Refunds automáticos
3. Webhook handling
4. Retry logic para fallos

### Fase 3 (Escalado)
1. Subscriptions
2. Invoicing
3. Multi-currency
4. Tax calculation
5. Compliance (PCI, etc)

## Security Notes

✅ Nunca almacenar plaintext card data (Stripe lo maneja)
✅ JWT tokens para autenticar requests de pago
✅ Verificar ownership antes de cobrar
✅ Stripe webhook signatures para confirmar eventos
✅ Has_active_payment_method: flag optimista, verificar en Stripe antes de cobrar

## Referencias

- **Stripe Setup Intents:** https://stripe.com/docs/payments/setup-intents
- **Payment Methods:** https://stripe.com/docs/api/payment_methods
- **Charges:** https://stripe.com/docs/api/charges
- **Webhooks:** https://stripe.com/docs/webhooks
