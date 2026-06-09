// Package auth verifica id_tokens de proveedores OIDC (Google, Apple, ...) y
// los normaliza a una Identity común que el handler usa para resolver/crear el
// usuario. El backend NUNCA confía en datos del cliente salvo el id_token, que
// se valida criptográficamente contra el proveedor.
package auth

import (
	"context"
	"fmt"

	"google.golang.org/api/idtoken"
)

// Identity es el resultado normalizado de verificar un id_token.
type Identity struct {
	Provider      string
	Sub           string // identificador estable del usuario dentro del proveedor
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

// ProviderVerifier verifica un id_token y extrae la identidad. Cada proveedor
// (Google hoy; Apple después) implementa esta interfaz.
type ProviderVerifier interface {
	Verify(ctx context.Context, idToken string) (*Identity, error)
}

// googleVerifier valida ID tokens de Google contra los JWKS públicos de Google
// y comprueba que la audience sea nuestro clientID (GOOGLE_CLIENT_ID).
type googleVerifier struct {
	clientID string
}

// NewGoogleVerifier crea el verificador de Google. clientID es el OAuth Client
// ID de Google Cloud, usado como audience esperada del token.
func NewGoogleVerifier(clientID string) ProviderVerifier {
	return &googleVerifier{clientID: clientID}
}

func (g *googleVerifier) Verify(ctx context.Context, idToken string) (*Identity, error) {
	payload, err := idtoken.Validate(ctx, idToken, g.clientID)
	if err != nil {
		return nil, fmt.Errorf("invalid google id_token: %w", err)
	}

	id := &Identity{
		Provider:      "google",
		Sub:           payload.Subject,
		Email:         claimString(payload.Claims, "email"),
		EmailVerified: claimBool(payload.Claims, "email_verified"),
		Name:          claimString(payload.Claims, "name"),
		Picture:       claimString(payload.Claims, "picture"),
	}
	return id, nil
}

// claimString extrae un claim string de forma segura.
func claimString(claims map[string]interface{}, key string) string {
	if v, ok := claims[key].(string); ok {
		return v
	}
	return ""
}

// claimBool extrae un claim booleano. Google lo emite como bool, pero algunos
// proveedores/flujos lo serializan como la cadena "true": se aceptan ambos.
func claimBool(claims map[string]interface{}, key string) bool {
	switch v := claims[key].(type) {
	case bool:
		return v
	case string:
		return v == "true"
	default:
		return false
	}
}
