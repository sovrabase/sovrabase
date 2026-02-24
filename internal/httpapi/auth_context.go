package httpapi

import (
	"context"
	"net/http"
	"strings"

	sharedcrypto "github.com/ketsuna-org/sovrabase/internal/shared/crypto"
)

type contextKey string

const authClaimsContextKey contextKey = "auth_claims"

func withClaims(ctx context.Context, claims sharedcrypto.Claims) context.Context {
	return context.WithValue(ctx, authClaimsContextKey, claims)
}

func claimsFromContext(ctx context.Context) (sharedcrypto.Claims, bool) {
	claims, ok := ctx.Value(authClaimsContextKey).(sharedcrypto.Claims)
	return claims, ok
}

func bearerTokenFromRequest(r *http.Request) string {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
