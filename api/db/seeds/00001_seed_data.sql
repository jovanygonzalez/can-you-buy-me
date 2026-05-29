-- +goose Up
-- +goose StatementBegin
-- =====================================================
-- Can You Buy Me - Seed Data (datos de ejemplo para desarrollo)
-- =====================================================
-- Se corre con goose -no-versioning (no cuenta como migración de schema).
-- Es re-ejecutable: limpia y recarga.

TRUNCATE payments, bids, audit_log, auctions, users RESTART IDENTITY CASCADE;

-- USERS (password_hash es un bcrypt de ejemplo de 'password123')
INSERT INTO users (email, name, password_hash, stripe_customer_id, has_active_payment_method, is_active)
VALUES
    ('maria.gonzalez@example.com', 'María González', '$2a$10$rZ8kHp9pXqZ8kHp9pXqZ8e', 'cus_example_maria', TRUE,  TRUE),
    ('juan.perez@example.com',     'Juan Pérez',     '$2a$10$rZ8kHp9pXqZ8kHp9pXqZ8e', 'cus_example_juan',  TRUE,  TRUE),
    ('ana.martinez@example.com',   'Ana Martínez',   '$2a$10$rZ8kHp9pXqZ8kHp9pXqZ8e', 'cus_example_ana',   TRUE,  TRUE),
    ('carlos.lopez@example.com',   'Carlos López',   '$2a$10$rZ8kHp9pXqZ8kHp9pXqZ8e', NULL,                FALSE, TRUE),
    ('laura.sanchez@example.com',  'Laura Sánchez',  '$2a$10$rZ8kHp9pXqZ8kHp9pXqZ8e', NULL,                FALSE, TRUE);

-- AUCTIONS (precios en pesos, DECIMAL). current_highest_bid NULL = sin pujas aún.
INSERT INTO auctions (title, description, category, base_price, current_highest_bid, image_urls, scheduled_at, started_at, status, condition)
VALUES
    ('iPhone 15 Pro Max 256GB', 'El último iPhone con titanio y cámara de 48MP', 'electronics', 20000.00, 20500.00, 'https://example.com/iphone15.jpg', NOW() - INTERVAL '1 hour',   NOW() - INTERVAL '1 hour',   'active',    'new'),
    ('PlayStation 5 Slim',      'Consola PS5 edición Slim con lector de discos', 'gaming',      10000.00, 10250.00, 'https://example.com/ps5.jpg',     NOW() - INTERVAL '2 hours',  NOW() - INTERVAL '2 hours',  'active',    'new'),
    ('MacBook Pro 14 M3',       'Laptop profesional con chip M3 Pro',            'electronics', 35000.00, NULL,     'https://example.com/macbook.jpg', NOW() + INTERVAL '1 day',    NULL,                        'draft',     'new'),
    ('Nintendo Switch OLED',    'Consola portátil con pantalla OLED',            'gaming',       7000.00,  7150.00, 'https://example.com/switch.jpg',  NOW() - INTERVAL '30 minutes', NOW() - INTERVAL '30 minutes', 'active', 'new'),
    ('AirPods Pro 2',           'Audífonos con cancelación de ruido',            'electronics',  5000.00,  5100.00, 'https://example.com/airpods.jpg', NOW() - INTERVAL '45 minutes', NOW() - INTERVAL '45 minutes', 'active', 'new');

-- BIDS (la puja válida más alta = current_highest_bid de la subasta)
INSERT INTO bids (auction_id, user_id, bid_amount, created_at, is_valid)
SELECT a.id, u.id, a.current_highest_bid, NOW() - INTERVAL '30 minutes', TRUE
FROM auctions a CROSS JOIN users u
WHERE a.title = 'iPhone 15 Pro Max 256GB' AND u.email = 'maria.gonzalez@example.com';

INSERT INTO bids (auction_id, user_id, bid_amount, created_at, is_valid)
SELECT a.id, u.id, a.current_highest_bid - 200.00, NOW() - INTERVAL '50 minutes', TRUE
FROM auctions a CROSS JOIN users u
WHERE a.title = 'iPhone 15 Pro Max 256GB' AND u.email = 'juan.perez@example.com';

INSERT INTO bids (auction_id, user_id, bid_amount, created_at, is_valid)
SELECT a.id, u.id, a.current_highest_bid, NOW() - INTERVAL '20 minutes', TRUE
FROM auctions a CROSS JOIN users u
WHERE a.title = 'PlayStation 5 Slim' AND u.email = 'ana.martinez@example.com';

INSERT INTO bids (auction_id, user_id, bid_amount, created_at, is_valid)
SELECT a.id, u.id, a.current_highest_bid, NOW() - INTERVAL '10 minutes', TRUE
FROM auctions a CROSS JOIN users u
WHERE a.title = 'Nintendo Switch OLED' AND u.email = 'carlos.lopez@example.com';

-- Marcar al mejor postor de cada subasta activa (highest_bidder_id)
UPDATE auctions a
SET highest_bidder_id = b.user_id
FROM bids b
WHERE b.auction_id = a.id
  AND b.bid_amount = a.current_highest_bid
  AND a.current_highest_bid IS NOT NULL;

-- PAYMENTS (cargo pendiente del mejor postor; amount es NOT NULL)
INSERT INTO payments (user_id, auction_id, stripe_payment_intent_id, amount, currency, status, is_manual, created_at)
SELECT a.highest_bidder_id, a.id,
       'pi_example_' || a.id || '_' || substr(md5(random()::text), 1, 10),
       a.current_highest_bid, 'MXN', 'pending', FALSE, NOW() - INTERVAL '5 minutes'
FROM auctions a
WHERE a.highest_bidder_id IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
TRUNCATE payments, bids, audit_log, auctions, users RESTART IDENTITY CASCADE;
-- +goose StatementEnd
