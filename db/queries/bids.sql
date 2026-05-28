-- name: CreateBid :one
INSERT INTO bids (
  auction_id,
  user_id,
  bid_amount,
  ip_address,
  user_agent,
  stream_seq,
  created_at,
  is_valid
)
VALUES ($1, $2, $3, $4, $5, $6, CURRENT_TIMESTAMP, true)
RETURNING *;

-- name: GetBidByID :one
SELECT * FROM bids
WHERE id = $1;

-- name: ListBidsByAuction :many
SELECT * FROM bids
WHERE auction_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetHighestBidForAuction :one
SELECT * FROM bids
WHERE auction_id = $1
ORDER BY bid_amount DESC, created_at ASC
LIMIT 1;

-- name: ListBidsByUser :many
SELECT * FROM bids
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountBidsByAuction :one
SELECT COUNT(*) as bid_count FROM bids
WHERE auction_id = $1 AND is_valid = true;

-- name: InvalidateBid :exec
UPDATE bids
SET is_valid = false, reject_reason = $1
WHERE id = $2;
