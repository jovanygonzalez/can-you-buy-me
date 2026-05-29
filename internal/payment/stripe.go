package payment

import (
	"fmt"
	"os"

	"github.com/stripe/stripe-go/v76"
	"github.com/stripe/stripe-go/v76/account"
	"github.com/stripe/stripe-go/v76/customer"
	"github.com/stripe/stripe-go/v76/setupintent"
)

// StripeClient envuelve el cliente de Stripe
type StripeClient struct {
	apiKey string
}

// NewStripeClient crea un nuevo cliente de Stripe
func NewStripeClient() (*StripeClient, error) {
	apiKey := os.Getenv("STRIPE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("STRIPE_API_KEY environment variable not set")
	}

	// Configurar la clave de API en el paquete stripe
	stripe.Key = apiKey

	return &StripeClient{
		apiKey: apiKey,
	}, nil
}

// VerifyConnection verifica que el cliente pueda conectarse a Stripe
// Retorna información de la cuenta si tiene éxito
func (sc *StripeClient) VerifyConnection() (*stripe.Account, error) {
	// Obtener información de la cuenta - esto valida la API key
	acc, err := account.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to verify Stripe connection: %w", err)
	}

	return acc, nil
}

// GetAccountInfo retorna información de la cuenta de Stripe
func (sc *StripeClient) GetAccountInfo() (*stripe.Account, error) {
	acc, err := account.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get account info: %w", err)
	}
	return acc, nil
}

// IsLiveMode verifica si está en modo live o test
func (sc *StripeClient) IsLiveMode() bool {
	// Si la API key comienza con "sk_live_", es modo live
	// Si comienza con "sk_test_", es modo test
	return len(sc.apiKey) > 7 && sc.apiKey[:7] == "sk_live"
}

// GetMode retorna "live" o "test"
func (sc *StripeClient) GetMode() string {
	if sc.IsLiveMode() {
		return "live"
	}
	return "test"
}

// CreateCustomer crea un nuevo Cliente en Stripe
func (sc *StripeClient) CreateCustomer(email, name string) (*stripe.Customer, error) {
	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Name:  stripe.String(name),
	}

	cust, err := customer.New(params)
	if err != nil {
		return nil, fmt.Errorf("failed to create Stripe customer: %w", err)
	}

	return cust, nil
}

// CreateSetupIntent crea un Setup Intent para guardar un método de pago
// Usage: "off_session" permite cobrar al usuario después sin que esté presente
func (sc *StripeClient) CreateSetupIntent(stripeCustomerID string) (*stripe.SetupIntent, error) {
	params := &stripe.SetupIntentParams{
		Customer:           stripe.String(stripeCustomerID),
		PaymentMethodTypes: []*string{stripe.String("card")},
		Usage:              stripe.String(string(stripe.SetupIntentUsageOffSession)),
	}

	intent, err := setupintent.New(params)
	if err != nil {
		return nil, fmt.Errorf("failed to create Setup Intent: %w", err)
	}

	return intent, nil
}
