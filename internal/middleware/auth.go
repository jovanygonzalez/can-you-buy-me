package middleware

import (
	"context"
	"strings"

	"github.com/can-you-buy-me/internal/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	// ContextUserID es la llave para almacenar el user_id en el context
	ContextUserID = "user_id"
	// ContextEmail es la llave para almacenar el email en el context
	ContextEmail = "email"
)

// AuthInterceptor valida JWT tokens en Unary RPCs
func AuthInterceptor(jwtManager *security.JWTManager) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Métodos públicos que no requieren autenticación
		publicMethods := map[string]bool{
			"/auth.v1.AuthService/Register": true,
			"/auth.v1.AuthService/Login":    true,
			"/health.v1.HealthService/Ping": true,
			"/health.v1.HealthService/Check": true,
		}

		// Si es un método público, saltarse validación
		if publicMethods[info.FullMethod] {
			return handler(ctx, req)
		}

		// Extraer token del metadata
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		tokens := md.Get("authorization")
		if len(tokens) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization token")
		}

		// El token viene como "Bearer <token>" — tolerar mayúsculas/minúsculas
		// y espacios alrededor ("bearer ", "BEARER ", etc.)
		tokenString := strings.TrimSpace(tokens[0])
		const bearerPrefix = "Bearer "
		if len(tokenString) >= len(bearerPrefix) && strings.EqualFold(tokenString[:len(bearerPrefix)], bearerPrefix) {
			tokenString = strings.TrimSpace(tokenString[len(bearerPrefix):])
		}

		// Validar token
		claims, err := jwtManager.ValidateToken(tokenString)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "invalid token")
		}

		// Agregar información del usuario al contexto
		ctx = context.WithValue(ctx, ContextUserID, claims.UserID)
		ctx = context.WithValue(ctx, ContextEmail, claims.Email)

		return handler(ctx, req)
	}
}

// GetUserIDFromContext extrae el user_id del contexto
func GetUserIDFromContext(ctx context.Context) (int32, bool) {
	userID, ok := ctx.Value(ContextUserID).(int32)
	return userID, ok
}

// GetEmailFromContext extrae el email del contexto
func GetEmailFromContext(ctx context.Context) (string, bool) {
	email, ok := ctx.Value(ContextEmail).(string)
	return email, ok
}
