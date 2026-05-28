package security

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims estructura de claims personalizados para el JWT
type Claims struct {
	UserID int32  `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// JWTManager maneja la generación y validación de JWT tokens
type JWTManager struct {
	secretKey string
	expiresIn time.Duration
}

// NewJWTManager crea un nuevo JWTManager.
// En producción (ENVIRONMENT=production) exige JWT_SECRET; en dev usa un
// secreto por defecto pero lo advierte para que no pase desapercibido.
func NewJWTManager() (*JWTManager, error) {
	secretKey := os.Getenv("JWT_SECRET")
	if secretKey == "" {
		if os.Getenv("ENVIRONMENT") == "production" {
			return nil, fmt.Errorf("JWT_SECRET environment variable must be set in production")
		}
		// Solo desarrollo: NUNCA usar este secreto en producción
		secretKey = "dev-secret-key-change-in-production"
		slog.Warn("JWT_SECRET not set; using insecure default key (development only)")
	}

	expiresIn := 24 * time.Hour // Token válido por 24 horas

	return &JWTManager{
		secretKey: secretKey,
		expiresIn: expiresIn,
	}, nil
}

// GenerateToken genera un nuevo JWT token
func (m *JWTManager) GenerateToken(userID int32, email string) (string, error) {
	claims := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.expiresIn)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "auction-api",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(m.secretKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateToken valida un JWT token y retorna los claims
func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			// Verificar que el método de firma es HMAC
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(m.secretKey), nil
		},
	)

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

// ExtractUserID extrae el user_id de un token
func (m *JWTManager) ExtractUserID(tokenString string) (int32, error) {
	claims, err := m.ValidateToken(tokenString)
	if err != nil {
		return 0, err
	}
	return claims.UserID, nil
}
