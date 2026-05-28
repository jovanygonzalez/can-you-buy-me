-- =====================================================
-- Can You Buy Me - Seed Data Example
-- MVP: Ejemplo de cómo cargar productos manualmente
-- =====================================================

-- NOTA: Este es un archivo de EJEMPLO para el MVP.
-- En producción, tú manualmente:
-- 1. Subes las imágenes a S3
-- 2. Copias las URLs de S3
-- 3. Insertas el producto aquí con todas las URLs
-- 4. Ejecutas este script para poblar la BD
-- 5. El backend carga todo en Redis para servir rápido

-- =====================================================
-- Ejemplo 1: Zapatilla de edición limitada
-- =====================================================
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
  is_featured,
  country_code
) VALUES (
  'Nike Air Jordan 1 Retro High OG "Chicago" 2024',
  'Limited Edition - Only 500 pairs worldwide',
  'Exclusive re-release of the iconic Chicago colorway. Brand new, unworn, factory sealed. Comes with original box and authentication certificate. Perfect for collectors and sneaker enthusiasts. These are one of the most coveted sneakers in the world, with historical significance dating back to 1985.',
  'sneakers',
  15000.00,
  'https://s3.amazonaws.com/can-you-buy-me/jordan1-chicago-1.jpg,https://s3.amazonaws.com/can-you-buy-me/jordan1-chicago-2.jpg,https://s3.amazonaws.com/can-you-buy-me/jordan1-chicago-3.jpg',
  'https://s3.amazonaws.com/can-you-buy-me/jordan1-chicago-thumb.jpg',
  '2024-06-15 20:00:00',
  'draft',
  'AJ1-CHICAGO-2024-001',
  'new',
  'verified',
  TRUE,
  'MX'
);

-- =====================================================
-- Ejemplo 2: Obra de arte digital
-- =====================================================
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
  is_featured,
  country_code
) VALUES (
  'Generative Art NFT - "Cosmic Dreams" Series #42',
  'High-resolution digital art certificate of authenticity included',
  'Part of the exclusive "Cosmic Dreams" series by renowned digital artist. Each piece is unique, generated using AI and manually curated. Includes HD download, print rights, and lifetime support. Digital art in high demand among collectors. Certificate of authenticity registered on blockchain.',
  'digital-art',
  5000.00,
  'https://s3.amazonaws.com/can-you-buy-me/cosmic-dreams-42-preview.jpg,https://s3.amazonaws.com/can-you-buy-me/cosmic-dreams-42-full.jpg',
  'https://s3.amazonaws.com/can-you-buy-me/cosmic-dreams-42-thumb.jpg',
  '2024-06-16 19:00:00',
  'draft',
  'COSMIC-DREAMS-42',
  'new',
  'verified',
  TRUE,
  'MX'
);

-- =====================================================
-- Ejemplo 3: Colectible / Comic
-- =====================================================
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
  is_featured,
  country_code
) VALUES (
  'Action Comics #1 Replica - First Edition Facsimile',
  'Pristine condition, professionally graded',
  'Limited edition facsimile of the historic first appearance of Superman. Published 1938, this is the most valuable comic ever printed. This replica is a museum-quality reproduction with original color printing on period paper stock. Comes in archival storage box with humidity control.',
  'collectibles',
  8500.00,
  'https://s3.amazonaws.com/can-you-buy-me/action-comics-1-front.jpg,https://s3.amazonaws.com/can-you-buy-me/action-comics-1-back.jpg,https://s3.amazonaws.com/can-you-buy-me/action-comics-1-detail.jpg',
  'https://s3.amazonaws.com/can-you-buy-me/action-comics-1-thumb.jpg',
  '2024-06-17 21:00:00',
  'draft',
  'ACTION-COMICS-REP-001',
  'new',
  'verified',
  FALSE,
  'MX'
);

-- =====================================================
-- Comentarios para el MVP
-- =====================================================
-- PASOS MANUALES ANTES DE EJECUTAR ESTE SCRIPT:
--
-- 1. Crear carpeta en S3: s3://can-you-buy-me/
-- 2. Subir todas las imágenes:
--    - jordan1-chicago-1.jpg
--    - jordan1-chicago-2.jpg
--    - jordan1-chicago-3.jpg
--    - jordan1-chicago-thumb.jpg
--    (Repetir para cosmic-dreams y action-comics)
--
-- 3. Copiar las URLs públicas de S3 (ej: https://s3.amazonaws.com/...)
--
-- 4. Reemplazar los valores en los INSERTs arriba con las URLs reales
--
-- 5. Ejecutar este script:
--    psql postgresql://root:root@localhost:5435/auction_db -f 002_seed_data.sql
--
-- 6. Verificar que los datos se cargaron:
--    SELECT id, title, base_price, status FROM auctions;
--
-- 7. El backend leerá esta data y la cacheará en Redis automáticamente
--    para servir rápido durante el drop.
--
-- PARA ACTUALIZAR UN PRODUCTO YA CARGADO:
-- UPDATE auctions SET status = 'active', started_at = NOW() WHERE id = 1;
--
-- PARA CERRAR UNA SUBASTA MANUALMENTE:
-- UPDATE auctions SET status = 'closed', ended_at = NOW(), winner_id = 5 WHERE id = 1;
--
-- Los bids se registran en tabla bids() cuando llegan a través de NATS en tiempo real.
