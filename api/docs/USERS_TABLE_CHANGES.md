# Users Table Changes

Cambios realizados en la tabla de usuarios para soportar Stripe Payment Methods.

## Cambio Realizado

### Antes (Original)
```sql
CREATE TABLE IF NOT EXISTS users (
  id SERIAL PRIMARY KEY,
  email VARCHAR(255) NOT NULL UNIQUE,
  name VARCHAR(255) NOT NULL,
  password_hash VARCHAR(255) NOT NULL,
  stripe_customer_id VARCHAR(255),
  stripe_payment_method_id VARCHAR(255),
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  is_active BOOLEAN DEFAULT TRUE
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_stripe_customer_id ON users(stripe_customer_id);
```

### Después (Actualizado)
```sql
CREATE TABLE IF NOT EXISTS users (
  id SERIAL PRIMARY KEY,

  -- Autenticación
  email VARCHAR(255) NOT NULL UNIQUE,
  name VARCHAR(255) NOT NULL,
  password_hash VARCHAR(255) NOT NULL,

  -- Stripe Payment Integration
  -- stripe_customer_id: ID del cliente en Stripe
  -- stripe_payment_method_id: ID del método de pago guardado
  -- has_active_payment_method: booleano para verificación rápida
  stripe_customer_id VARCHAR(255),
  stripe_payment_method_id VARCHAR(255),
  has_active_payment_method BOOLEAN DEFAULT FALSE,  -- ← NUEVO

  -- Metadata
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  is_active BOOLEAN DEFAULT TRUE
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_stripe_customer_id ON users(stripe_customer_id);
CREATE INDEX idx_users_has_active_payment ON users(has_active_payment_method);  -- ← NUEVO
```

## Cambios Detallados

### 1. Nuevo Campo: `has_active_payment_method`

```sql
has_active_payment_method BOOLEAN DEFAULT FALSE
```

**Propósito:**
- Flag booleano que indica si el usuario tiene un método de pago válido
- DEFAULT FALSE: nuevo usuario no puede pujar hasta guardar tarjeta
- Permite validaciones rápidas sin consultar Stripe API
- Mejora performance en queries de validación

**Casos de uso:**
```sql
-- Verificar si usuario puede pujar
SELECT has_active_payment_method FROM users WHERE id = 1;
-- Resultado: true/false → Permitir/denegar puja

-- Listar usuarios con pago activo
SELECT * FROM users WHERE has_active_payment_method = true;

-- Validar antes de permitir cobro
IF user.has_active_payment_method == true {
  // Proceder con cobro
}
```

### 2. Nuevo Índice: `idx_users_has_active_payment`

```sql
CREATE INDEX idx_users_has_active_payment ON users(has_active_payment_method);
```

**Propósito:**
- Acelera queries que filtran por este campo
- Evita table scans en búsquedas de usuarios con pago activo
- Importante para listar "usuarios que pueden pujar"

**Performance:**
- Sin índice: O(n) - escanea toda la tabla
- Con índice: O(log n) - búsqueda binaria

## Cambios en SQL Queries

### Nuevos Queries Agregados

**`UpdateUserPaymentMethodStatus`**
```sql
-- name: UpdateUserPaymentMethodStatus :one
UPDATE users
SET has_active_payment_method = $1, updated_at = CURRENT_TIMESTAMP
WHERE id = $2
RETURNING *;
```
Actualiza solo el flag de pago activo.

**`UpdateUserStripePaymentMethodWithStatus`**
```sql
-- name: UpdateUserStripePaymentMethodWithStatus :one
UPDATE users
SET
  stripe_payment_method_id = $1,
  has_active_payment_method = $2,
  updated_at = CURRENT_TIMESTAMP
WHERE id = $3
RETURNING *;
```
Actualiza payment method ID y flag simultáneamente.

**`ListUsersWithActivePayment`**
```sql
-- name: ListUsersWithActivePayment :many
SELECT * FROM users
WHERE is_active = true AND has_active_payment_method = true
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;
```
Lista usuarios activos que tienen pago configurado.

## Por Qué Estos Cambios

### 1. MVP Philosophy
En el MVP **no automatizamos cobros complejos**. Para cobrar:
1. Admin cierra subasta manualmente
2. Verifica que ganador tiene `has_active_payment_method = true`
3. Cobra manualmente desde Stripe Dashboard

### 2. Performance
Sin el flag booleano, tendríamos que:
```go
// Cada vez que verificamos si usuario puede pujar:
stripe.Customer, err := client.Customers.Get(user.StripeCustomerID)
if stripe.Customer.PaymentMethods.Count > 0 {
  // Puede pujar
}
```

Con el flag:
```go
// Rápido - acceso directo a BD
if user.HasActivePaymentMethod {
  // Puede pujar
}
```

### 3. Desacoplamiento de Stripe
Si Stripe está down, el app sigue funcionando:
- Lectura de `has_active_payment_method` funciona
- Pedir tarjeta nueva requeriría Stripe

## Pasos para Aplicar en Desarrollo

Como indicaste, **todo está en desarrollo y no se ha corrido nada en DB**, así que:

### 1. Script está listo
El archivo `api/sql/001_init.sql` ya contiene:
- ✅ Campo `has_active_payment_method`
- ✅ Índice `idx_users_has_active_payment`
- ✅ Comentarios explicativos

### 2. Cuando ejecutes primera vez
```bash
make docker-up
make db-migrate
```

Se creará la tabla con todos los campos incluidos.

### 3. Queries están listos
`db/queries/users.sql` contiene todos los nuevos queries:
```bash
make sqlc
```

Generará automáticamente los métodos Go.

## Compatibilidad con AuthService

El AuthService actual:
```go
func (h *AuthHandler) Register(ctx context.Context, req *pb.RegisterRequest) {
  // Crea usuario con:
  // - email, name, password_hash
  // - has_active_payment_method = FALSE (default)
  // - stripe_customer_id = NULL
  // - stripe_payment_method_id = NULL
  user, err := h.queries.CreateUser(ctx, db.CreateUserParams{
    Email:        email,
    Name:         name,
    PasswordHash: passwordHash,
  })
}
```

El usuario NO PUEDE PUJAR hasta que:
1. Llama a `InitializeStripePayment` (futuro endpoint)
2. Completa Setup Intent
3. Backend actualiza `has_active_payment_method = true`

## Resumen de Cambios

| Aspecto | Antes | Después |
|---------|-------|---------|
| Campos de Stripe | 2 (customer_id, payment_method_id) | 3 (+ has_active_payment_method) |
| Índices | 2 | 3 (+ idx_users_has_active_payment) |
| Queries | 7 | 10 (+ 3 nuevos) |
| Validación de Pago | Consulta Stripe | Flag booleano (rápido) |
| Estado Default | N/A | FALSE (no puede pujar) |

## Archivos Modificados

- ✅ `api/sql/001_init.sql` - Tabla actualizada con nuevo campo e índice
- ✅ `db/queries/users.sql` - 3 nuevos queries para pago
- ✅ `STRIPE_INTEGRATION.md` - Documentación de integración (nuevo)

## Sin Breaking Changes

✅ No elimina campos existentes
✅ No cambia estructura de autenticación
✅ Compatible con AuthService actual
✅ Forward compatible para futuras features

## Próximas Implementaciones

1. **`InitializeStripePayment` endpoint**
   - Crea Stripe Customer
   - Crea Setup Intent
   - Retorna client_secret para Flutter

2. **`ConfirmPaymentMethod` endpoint**
   - Confirma que payment method se guardó
   - Actualiza `has_active_payment_method = true`

3. **`ChargeWinner` endpoint (opcional)**
   - Cobra automáticamente post-subasta
   - O usar Dashboard Stripe manualmente

Ver `STRIPE_INTEGRATION.md` para detalles de cada endpoint.
