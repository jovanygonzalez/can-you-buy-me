package handlers

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	db "github.com/can-you-buy-me/db/sqlc"
	"github.com/can-you-buy-me/internal/security"
	pb "github.com/can-you-buy-me/pkg/gen/auth/v1"
)

// AuthHandler implementa el servicio gRPC de autenticación
type AuthHandler struct {
	pb.UnimplementedAuthServiceServer
	queries    *db.Queries
	jwtManager *security.JWTManager
}

// NewAuthHandler crea un nuevo AuthHandler
func NewAuthHandler(queries *db.Queries, jwtManager *security.JWTManager) *AuthHandler {
	return &AuthHandler{
		queries:    queries,
		jwtManager: jwtManager,
	}
}

// Register registra un nuevo usuario
func (h *AuthHandler) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	// Validar entrada
	if req.Email == "" || req.Name == "" || req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "email, name, and password are required")
	}

	if len(req.Password) < 6 {
		return nil, status.Error(codes.InvalidArgument, "password must be at least 6 characters")
	}

	// Verificar si el usuario ya existe.
	// GetUserByEmail retorna (User, error) por valor: si err == nil, el usuario existe.
	if _, err := h.queries.GetUserByEmail(ctx, req.Email); err == nil {
		return nil, status.Error(codes.AlreadyExists, "user with this email already exists")
	}

	// Hash de la contraseña
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to hash password")
	}

	// Crear usuario en la base de datos
	user, err := h.queries.CreateUser(ctx, db.CreateUserParams{
		Email:        req.Email,
		Name:         req.Name,
		PasswordHash: string(hashedPassword),
	})
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to create user: %v", err))
	}

	return &pb.RegisterResponse{
		UserId:  user.ID,
		Email:   user.Email,
		Message: "User registered successfully",
	}, nil
}

// Login autentica un usuario y retorna un JWT token
func (h *AuthHandler) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	// Validar entrada
	if req.Email == "" || req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "email and password are required")
	}

	// Obtener usuario de la base de datos
	user, err := h.queries.GetUserByEmail(ctx, req.Email)
	if err != nil {
		// No revelar si el usuario existe o no
		return nil, status.Error(codes.Unauthenticated, "invalid email or password")
	}

	// Verificar contraseña
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid email or password")
	}

	// Generar JWT token
	token, err := h.jwtManager.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate token")
	}

	return &pb.LoginResponse{
		JwtToken: token,
		UserId:   user.ID,
		Email:    user.Email,
	}, nil
}
