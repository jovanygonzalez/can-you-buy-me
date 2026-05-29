package handlers

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	db "github.com/can-you-buy-me/db/sqlc"
	"github.com/can-you-buy-me/internal/middleware"
	"github.com/can-you-buy-me/internal/payment"
	pb "github.com/can-you-buy-me/pkg/gen/payment/v1"
)

// PaymentHandler maneja las operaciones de pago y Stripe
type PaymentHandler struct {
	pb.UnimplementedPaymentServiceServer
	queries      *db.Queries
	stripeClient *payment.StripeClient
}

// NewPaymentHandler crea un nuevo PaymentHandler
func NewPaymentHandler(queries *db.Queries, stripeClient *payment.StripeClient) *PaymentHandler {
	return &PaymentHandler{
		queries:      queries,
		stripeClient: stripeClient,
	}
}

// InitializeStripePayment crea un Stripe Customer (si no existe) y un Setup Intent
// El cliente (Flutter) usa el client_secret para confirmar la tarjeta con Stripe.js
func (h *PaymentHandler) InitializeStripePayment(ctx context.Context, req *pb.InitializeStripePaymentRequest) (*pb.InitializeStripePaymentResponse, error) {
	// Extraer user_id del contexto JWT (inyectado por AuthInterceptor)
	userID, ok := middleware.GetUserIDFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "user_id not found in context")
	}

	// Obtener información del usuario
	user, err := h.queries.GetUserByID(ctx, userID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}

	// stripe_customer_id es nullable en la BD → sqlc lo expone como pgtype.Text
	var stripeCustomerID string
	if user.StripeCustomerID.Valid {
		stripeCustomerID = user.StripeCustomerID.String
	}

	// Si el usuario no tiene un Customer en Stripe, crearlo
	if stripeCustomerID == "" {
		cust, err := h.stripeClient.CreateCustomer(user.Email, user.Name)
		if err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to create Stripe customer: %v", err))
		}

		stripeCustomerID = cust.ID

		// Guardar el Customer ID en la base de datos
		_, err = h.queries.UpdateUserStripeCustomer(ctx, db.UpdateUserStripeCustomerParams{
			StripeCustomerID: pgtype.Text{String: stripeCustomerID, Valid: true},
			ID:               userID,
		})
		if err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to save Stripe customer ID: %v", err))
		}
	}

	// Crear un Setup Intent para que el cliente pueda guardar su tarjeta
	intent, err := h.stripeClient.CreateSetupIntent(stripeCustomerID)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to create Setup Intent: %v", err))
	}

	return &pb.InitializeStripePaymentResponse{
		ClientSecret:      intent.ClientSecret,
		StripeCustomerId:  stripeCustomerID,
		Message:           "Setup Intent created successfully",
	}, nil
}
