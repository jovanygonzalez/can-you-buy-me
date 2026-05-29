-- +goose Up
-- +goose StatementBegin
-- =====================================================
-- Can You Buy Me - Initial Schema
-- MVP: Users, Products/Auctions, Bids (audit trail)
-- =====================================================

-- =====================================================
-- USERS TABLE
-- =====================================================
CREATE TABLE IF NOT EXISTS users (
  id SERIAL PRIMARY KEY,

  -- Autenticación
  email VARCHAR(255) NOT NULL UNIQUE,
  name VARCHAR(255) NOT NULL,
  password_hash VARCHAR(255) NOT NULL,

  -- Stripe Payment Integration
  stripe_customer_id VARCHAR(255),
  stripe_payment_method_id VARCHAR(255),
  has_active_payment_method BOOLEAN DEFAULT FALSE,

  -- Metadata
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  is_active BOOLEAN DEFAULT TRUE
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_stripe_customer_id ON users(stripe_customer_id);
CREATE INDEX idx_users_has_active_payment ON users(has_active_payment_method);

-- =====================================================
-- AUCTIONS TABLE (Productos en Subasta)
-- =====================================================
CREATE TABLE IF NOT EXISTS auctions (
  id SERIAL PRIMARY KEY,

  -- Información básica del producto
  title VARCHAR(255) NOT NULL,
  subtitle VARCHAR(255),
  description TEXT,
  category VARCHAR(100),

  -- Precios y estado de subasta
  base_price DECIMAL(12, 2) NOT NULL,
  current_highest_bid DECIMAL(12, 2),
  final_price DECIMAL(12, 2),

  -- Imágenes (URLs a S3)
  image_urls TEXT,
  thumbnail_url VARCHAR(500),

  -- Timing de la subasta
  scheduled_at TIMESTAMP NOT NULL,
  started_at TIMESTAMP,
  ended_at TIMESTAMP,

  -- Estado: draft | active | closed | cancelled
  status VARCHAR(50) DEFAULT 'draft',

  -- Ganador y resultado
  winner_id INTEGER REFERENCES users(id),
  highest_bidder_id INTEGER REFERENCES users(id),

  -- Campos adicionales de producto
  sku VARCHAR(100) UNIQUE,
  condition VARCHAR(50),
  authentication_status VARCHAR(100),

  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  created_by INTEGER REFERENCES users(id),

  -- Para futuro escalado
  country_code VARCHAR(2) DEFAULT 'MX',
  is_featured BOOLEAN DEFAULT FALSE
);

CREATE INDEX idx_auctions_status ON auctions(status);
CREATE INDEX idx_auctions_scheduled_at ON auctions(scheduled_at);
CREATE INDEX idx_auctions_winner_id ON auctions(winner_id);
CREATE INDEX idx_auctions_highest_bidder_id ON auctions(highest_bidder_id);
CREATE INDEX idx_auctions_category ON auctions(category);

-- =====================================================
-- BIDS TABLE (Auditoría de Pujas)
-- =====================================================
CREATE TABLE IF NOT EXISTS bids (
  id BIGSERIAL PRIMARY KEY,

  auction_id INTEGER NOT NULL REFERENCES auctions(id),
  user_id INTEGER NOT NULL REFERENCES users(id),

  bid_amount DECIMAL(12, 2) NOT NULL,

  -- Metadata de la puja
  ip_address VARCHAR(45),
  user_agent TEXT,

  -- Secuencia global del Stream de JetStream (orden cronológico autoritativo).
  -- La puebla el audit consumer.
  stream_seq BIGINT UNIQUE,

  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

  is_valid BOOLEAN DEFAULT TRUE,
  reject_reason VARCHAR(500)
);

CREATE INDEX idx_bids_auction_id ON bids(auction_id);
CREATE INDEX idx_bids_user_id ON bids(user_id);
CREATE INDEX idx_bids_created_at ON bids(created_at DESC);
CREATE INDEX idx_bids_auction_created ON bids(auction_id, created_at DESC);

-- =====================================================
-- AUDIT_LOG TABLE (Traza general de operaciones críticas)
-- =====================================================
CREATE TABLE IF NOT EXISTS audit_log (
  id BIGSERIAL PRIMARY KEY,

  event_type VARCHAR(100) NOT NULL,

  user_id INTEGER REFERENCES users(id),
  auction_id INTEGER REFERENCES auctions(id),

  description TEXT,
  metadata JSONB,

  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_audit_log_event_type ON audit_log(event_type);
CREATE INDEX idx_audit_log_user_id ON audit_log(user_id);
CREATE INDEX idx_audit_log_auction_id ON audit_log(auction_id);
CREATE INDEX idx_audit_log_created_at ON audit_log(created_at DESC);

-- =====================================================
-- PAYMENTS TABLE (Registro de transacciones Stripe)
-- =====================================================
CREATE TABLE IF NOT EXISTS payments (
  id BIGSERIAL PRIMARY KEY,

  user_id INTEGER NOT NULL REFERENCES users(id),
  auction_id INTEGER NOT NULL REFERENCES auctions(id),

  stripe_charge_id VARCHAR(255) UNIQUE,
  stripe_payment_intent_id VARCHAR(255) UNIQUE,

  amount DECIMAL(12, 2) NOT NULL,
  currency VARCHAR(3) DEFAULT 'MXN',

  status VARCHAR(50),
  error_message TEXT,

  is_manual BOOLEAN DEFAULT FALSE,

  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_payments_user_id ON payments(user_id);
CREATE INDEX idx_payments_auction_id ON payments(auction_id);
CREATE INDEX idx_payments_stripe_charge_id ON payments(stripe_charge_id);
CREATE INDEX idx_payments_status ON payments(status);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS bids;
DROP TABLE IF EXISTS auctions;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
