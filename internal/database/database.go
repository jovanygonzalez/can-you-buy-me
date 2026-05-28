package database

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB envuelve pgxpool para las operaciones de base de datos
type DB struct {
	*pgxpool.Pool
}

// Option configura el pgxpool.Config
type Option func(*pgxpool.Config)

// New crea una nueva conexión a PostgreSQL
func New(opts ...Option) (*DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return buildPool(ctx, opts...)
}

// buildPool centraliza la construcción del pool de conexiones
func buildPool(ctx context.Context, opts ...Option) (*DB, error) {
	// Leer variables de entorno
	host := getEnvOrDefault("DB_HOST", "localhost")
	port := getEnvOrDefault("DB_PORT", "5435")
	user := getEnvOrDefault("DB_USER", "root")
	password := getEnvOrDefault("DB_PASSWORD", "root")
	dbname := getEnvOrDefault("DB_NAME", "auction_db")
	sslmode := getEnvOrDefault("DB_SSL_MODE", "disable")

	// Construir DSN usando url.URL para escapar caracteres especiales
	dsnURL := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   net.JoinHostPort(host, port),
		Path:   "/" + dbname,
	}
	q := dsnURL.Query()
	q.Set("sslmode", sslmode)
	q.Set("application_name", "auction-api")
	dsnURL.RawQuery = q.Encode()

	// Parsear la configuración
	cfg, err := pgxpool.ParseConfig(dsnURL.String())
	if err != nil {
		return nil, fmt.Errorf("error parsing database config: %w", err)
	}

	// Configurar pool
	cfg.MaxConns = int32(getEnvAsInt("DB_MAX_CONNECTIONS", 25))
	cfg.MinConns = int32(getEnvAsInt("DB_MAX_IDLE_CONNECTIONS", 5))
	cfg.MaxConnLifetime = 1 * time.Hour
	cfg.MaxConnLifetimeJitter = 5 * time.Minute
	cfg.MaxConnIdleTime = 5 * time.Minute

	// Aplicar opciones personalizadas
	for _, opt := range opts {
		opt(cfg)
	}

	// Crear el pool
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("error creating connection pool: %w", err)
	}

	// Verificar conectividad con ping
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer pingCancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("error pinging database: %w", err)
	}

	return &DB{pool}, nil
}

// Close cierra la conexión a la base de datos
func (db *DB) Close() {
	if db != nil && db.Pool != nil {
		db.Pool.Close()
	}
}

// getEnvOrDefault retorna una variable de entorno o un valor por defecto
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt retorna una variable de entorno como entero
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if n, err := strconv.Atoi(value); err == nil {
			return n
		}
	}
	return defaultValue
}
