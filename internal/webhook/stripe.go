package webhook

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/webhook"
)

// StripeWebhookHandler maneja los webhooks de Stripe
type StripeWebhookHandler struct {
	webhookSecret string
}

// NewStripeWebhookHandler crea un nuevo handler de webhooks de Stripe
// Si STRIPE_WEBHOOK_SECRET no está configurada, retorna error
func NewStripeWebhookHandler() (*StripeWebhookHandler, error) {
	secret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	if secret == "" {
		return nil, fmt.Errorf("STRIPE_WEBHOOK_SECRET environment variable not set")
	}

	return &StripeWebhookHandler{
		webhookSecret: secret,
	}, nil
}

// Handle procesa los webhooks de Stripe
func (h *StripeWebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	// Solo POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Leer el body crudo (necesario para verificar la firma)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("Failed to read webhook body", "error", err)
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	// Obtener la firma del header
	sigHeader := r.Header.Get("Stripe-Signature")

	// Verificar la firma con webhook.ConstructEvent
	event, err := webhook.ConstructEvent(body, sigHeader, h.webhookSecret)
	if err != nil {
		slog.Error("Failed to verify webhook signature", "error", err)
		http.Error(w, "Bad signature", http.StatusBadRequest)
		return
	}

	// Loggear que se recibió un webhook
	slog.Info("Stripe webhook received",
		"event_type", event.Type,
		"event_id", event.ID,
	)

	// Despachar por tipo de evento
	switch event.Type {
	case "setup_intent.succeeded":
		h.handleSetupIntentSucceeded(event)

	case "setup_intent.setup_failed":
		h.handleSetupIntentFailed(event)

	default:
		// Loggear pero no fallar — Stripe reintenta si recibe error
		slog.Info("Unhandled Stripe event type", "event_type", event.Type)
	}

	// Siempre responder 200 OK a Stripe
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// handleSetupIntentSucceeded procesa el evento setup_intent.succeeded
func (h *StripeWebhookHandler) handleSetupIntentSucceeded(event stripe.Event) {
	var setupIntent stripe.SetupIntent

	// Unmarshalar el raw JSON al tipo SetupIntent
	if err := json.Unmarshal(event.Data.Raw, &setupIntent); err != nil {
		slog.Error("Failed to parse setup_intent.succeeded event",
			"error", err,
			"event_id", event.ID,
		)
		return
	}

	// Customer y PaymentMethod son punteros: pueden venir nil en el payload
	// (p.ej. SetupIntent sin customer). Acceder a .ID sin chequear = panic.
	var customerID, paymentMethodID string
	if setupIntent.Customer != nil {
		customerID = setupIntent.Customer.ID
	}
	if setupIntent.PaymentMethod != nil {
		paymentMethodID = setupIntent.PaymentMethod.ID
	}

	// Loggear los detalles del Setup Intent
	slog.Info("setup_intent.succeeded",
		"intent_id", setupIntent.ID,
		"customer_id", customerID,
		"status", setupIntent.Status,
		"payment_method", paymentMethodID,
		"event_id", event.ID,
	)

	// En MVP Fase 1: solo loggear
	// Fase posterior: actualizar has_active_payment_method en PostgreSQL
}

// handleSetupIntentFailed procesa el evento setup_intent.setup_failed
func (h *StripeWebhookHandler) handleSetupIntentFailed(event stripe.Event) {
	var setupIntent stripe.SetupIntent

	if err := json.Unmarshal(event.Data.Raw, &setupIntent); err != nil {
		slog.Error("Failed to parse setup_intent.setup_failed event",
			"error", err,
			"event_id", event.ID,
		)
		return
	}

	// Customer y LastSetupError son punteros: chequear nil antes de .ID/.Code
	var customerID string
	if setupIntent.Customer != nil {
		customerID = setupIntent.Customer.ID
	}
	var errorCode stripe.ErrorCode
	var errorMessage string
	if setupIntent.LastSetupError != nil {
		errorCode = setupIntent.LastSetupError.Code
		errorMessage = setupIntent.LastSetupError.Message
	}

	// Loggear el fallo
	slog.Warn("setup_intent.setup_failed",
		"intent_id", setupIntent.ID,
		"customer_id", customerID,
		"status", setupIntent.Status,
		"error_code", errorCode,
		"error_message", errorMessage,
		"event_id", event.ID,
	)

	// En futuro: notificar al usuario del fallo
}
