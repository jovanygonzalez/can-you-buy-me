-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1;

-- name: CreateUser :one
INSERT INTO users (email, name, password_hash, created_at, updated_at)
VALUES ($1, $2, $3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
RETURNING *;

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
