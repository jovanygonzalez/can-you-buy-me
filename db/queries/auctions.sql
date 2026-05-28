-- name: GetAuctionByID :one
SELECT * FROM auctions
WHERE id = $1;

-- name: ListActiveAuctions :many
SELECT * FROM auctions
WHERE status IN ('draft', 'active')
ORDER BY scheduled_at DESC
LIMIT $1 OFFSET $2;

-- name: ListAuctionsByStatus :many
SELECT * FROM auctions
WHERE status = $1
ORDER BY scheduled_at DESC
LIMIT $2 OFFSET $3;

-- name: CreateAuction :one
INSERT INTO auctions (
  title,
  subtitle,
  description,
  category,
  base_price,
  image_urls,
  thumbnail_url,
  scheduled_at,
  status,
  sku,
  condition,
  authentication_status,
  created_at,
  updated_at,
  created_by,
  country_code,
  is_featured
)
VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
  $11, $12, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, $13, $14, $15
)
RETURNING *;

-- name: UpdateAuctionStatus :one
UPDATE auctions
SET status = $1, updated_at = CURRENT_TIMESTAMP
WHERE id = $2
RETURNING *;

-- name: UpdateAuctionStarted :one
UPDATE auctions
SET started_at = CURRENT_TIMESTAMP, status = 'active', updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: UpdateAuctionClosed :one
UPDATE auctions
SET
  status = 'closed',
  ended_at = CURRENT_TIMESTAMP,
  winner_id = $2,
  final_price = $3,
  updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: UpdateCurrentHighestBid :one
UPDATE auctions
SET
  current_highest_bid = $1,
  highest_bidder_id = $2,
  updated_at = CURRENT_TIMESTAMP
WHERE id = $3
RETURNING *;

-- name: GetAuctionForBidding :one
SELECT id, current_highest_bid, status, scheduled_at, ended_at
FROM auctions
WHERE id = $1 AND (status = 'active' OR status = 'draft');
