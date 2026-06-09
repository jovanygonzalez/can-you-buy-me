-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1;

-- name: CreateUser :one
INSERT INTO users (email, name, avatar_url, created_at, updated_at)
VALUES ($1, $2, $3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
RETURNING *;

-- name: UpdateUserLastLogin :exec
UPDATE users
SET last_login_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE id = $1;

-- =====================================================
-- IDENTIDADES OAuth (Google, Apple, ...)
-- =====================================================

-- name: GetUserByProviderID :one
-- El lookup principal del login OAuth: dado (provider, sub), devuelve el usuario.
SELECT u.* FROM users u
JOIN user_identities i ON i.user_id = u.id
WHERE i.provider = $1 AND i.provider_user_id = $2;

-- name: GetIdentity :one
SELECT * FROM user_identities
WHERE provider = $1 AND provider_user_id = $2;

-- name: ListIdentitiesByUser :many
SELECT * FROM user_identities
WHERE user_id = $1
ORDER BY created_at;

-- name: CreateUserIdentity :one
INSERT INTO user_identities (user_id, provider, provider_user_id, email_at_provider)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: TouchIdentityLastLogin :exec
UPDATE user_identities
SET last_login_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE provider = $1 AND provider_user_id = $2;

-- name: DeleteUserIdentity :exec
-- Para "desvincular" un proveedor de una cuenta.
DELETE FROM user_identities
WHERE user_id = $1 AND provider = $2;

-- name: UpdateUserStripeCustomer :one
UPDATE users
SET stripe_customer_id = $1, updated_at = CURRENT_TIMESTAMP
WHERE id = $2
RETURNING *;

-- name: UpdateUserStripePaymentMethod :one
UPDATE users
SET stripe_payment_method_id = $1, updated_at = CURRENT_TIMESTAMP
WHERE id = $2
RETURNING *;

-- name: UpdateUserPaymentMethodStatus :one
UPDATE users
SET has_active_payment_method = $1, updated_at = CURRENT_TIMESTAMP
WHERE id = $2
RETURNING *;

-- name: UpdateUserStripePaymentMethodWithStatus :one
UPDATE users
SET
  stripe_payment_method_id = $1,
  has_active_payment_method = $2,
  updated_at = CURRENT_TIMESTAMP
WHERE id = $3
RETURNING *;

-- name: ListUsers :many
SELECT * FROM users
WHERE is_active = true
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListUsersWithActivePayment :many
SELECT * FROM users
WHERE is_active = true AND has_active_payment_method = true
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: DeactivateUser :exec
UPDATE users
SET is_active = false, updated_at = CURRENT_TIMESTAMP
WHERE id = $1;
