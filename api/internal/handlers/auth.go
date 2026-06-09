package handlers

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	db "github.com/can-you-buy-me/db/sqlc"
	"github.com/can-you-buy-me/internal/auth"
	"github.com/can-you-buy-me/internal/security"
	pb "github.com/can-you-buy-me/pkg/gen/auth/v1"
)

// AuthHandler implementa el servicio gRPC de autenticación.
// Auth es OAuth-only: el cliente trae un id_token de un proveedor y aquí se
// intercambia por un JWT propio.
type AuthHandler struct {
	pb.UnimplementedAuthServiceServer
	pool       *pgxpool.Pool
	queries    *db.Queries
	jwtManager *security.JWTManager
	verifiers  map[string]auth.ProviderVerifier
}

// NewAuthHandler crea un nuevo AuthHandler. pool se usa para abrir la
// transacción del upsert; verifiers mapea "google"|"apple"|... a su verificador.
func NewAuthHandler(
	pool *pgxpool.Pool,
	queries *db.Queries,
	jwtManager *security.JWTManager,
	verifiers map[string]auth.ProviderVerifier,
) *AuthHandler {
	return &AuthHandler{
		pool:       pool,
		queries:    queries,
		jwtManager: jwtManager,
		verifiers:  verifiers,
	}
}

// LoginWithProvider verifica el id_token del proveedor, resuelve (o crea) el
// usuario con vinculación por email verificado, y devuelve un JWT propio.
func (h *AuthHandler) LoginWithProvider(ctx context.Context, req *pb.LoginWithProviderRequest) (*pb.LoginResponse, error) {
	if req.Provider == "" || req.IdToken == "" {
		return nil, status.Error(codes.InvalidArgument, "provider and id_token are required")
	}

	verifier, ok := h.verifiers[req.Provider]
	if !ok {
		return nil, status.Errorf(codes.Unimplemented, "provider %q is not supported", req.Provider)
	}

	// El id_token se valida criptográficamente contra el proveedor.
	identity, err := verifier.Verify(ctx, req.IdToken)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid id_token")
	}

	// La vinculación por email exige un correo verificado por el proveedor.
	if identity.Email == "" {
		return nil, status.Error(codes.PermissionDenied, "provider did not return an email")
	}
	if !identity.EmailVerified {
		return nil, status.Error(codes.PermissionDenied, "provider email is not verified")
	}

	user, isNew, err := h.resolveUser(ctx, identity)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to resolve user: %v", err))
	}

	token, err := h.jwtManager.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate token")
	}

	return &pb.LoginResponse{
		JwtToken:  token,
		UserId:    user.ID,
		Email:     user.Email,
		IsNewUser: isNew,
	}, nil
}

// resolveUser aplica la política de cuentas dentro de una transacción:
//  1. Si la identidad (provider, sub) ya existe → login normal.
//  2. Si no, pero hay un usuario con ese email verificado → VINCULA (nueva fila
//     en user_identities), sin crear cuenta duplicada.
//  3. Si tampoco existe el usuario → lo crea y vincula la identidad.
//
// Devuelve el usuario y si fue creado en este login (is_new_user).
func (h *AuthHandler) resolveUser(ctx context.Context, identity *auth.Identity) (db.User, bool, error) {
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return db.User{}, false, err
	}
	defer tx.Rollback(ctx)

	q := h.queries.WithTx(tx)

	// 1) Identidad ya conocida → login normal.
	user, err := q.GetUserByProviderID(ctx, db.GetUserByProviderIDParams{
		Provider:       identity.Provider,
		ProviderUserID: identity.Sub,
	})
	if err == nil {
		if err := q.TouchIdentityLastLogin(ctx, db.TouchIdentityLastLoginParams{
			Provider:       identity.Provider,
			ProviderUserID: identity.Sub,
		}); err != nil {
			return db.User{}, false, err
		}
		if err := q.UpdateUserLastLogin(ctx, user.ID); err != nil {
			return db.User{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return db.User{}, false, err
		}
		return user, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return db.User{}, false, err
	}

	// 2) Identidad nueva: ¿existe un usuario con ese email? → vincular.
	isNew := false
	user, err = q.GetUserByEmail(ctx, identity.Email)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return db.User{}, false, err
		}
		// 3) No existe usuario → crearlo.
		user, err = q.CreateUser(ctx, db.CreateUserParams{
			Email:     identity.Email,
			Name:      identity.Name,
			AvatarUrl: textOrNull(identity.Picture),
		})
		if err != nil {
			return db.User{}, false, err
		}
		isNew = true
	}

	// Vincular la identidad del proveedor al usuario (existente o recién creado).
	if _, err := q.CreateUserIdentity(ctx, db.CreateUserIdentityParams{
		UserID:          user.ID,
		Provider:        identity.Provider,
		ProviderUserID:  identity.Sub,
		EmailAtProvider: textOrNull(identity.Email),
	}); err != nil {
		return db.User{}, false, err
	}

	if err := q.UpdateUserLastLogin(ctx, user.ID); err != nil {
		return db.User{}, false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return db.User{}, false, err
	}
	return user, isNew, nil
}

// textOrNull convierte un string a pgtype.Text, dejándolo NULL si está vacío.
func textOrNull(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}
