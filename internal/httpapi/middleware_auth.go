package httpapi

import (
	"net/http"

	sharedcrypto "github.com/ketsuna-org/sovrabase/internal/shared/crypto"
)

func requireAdminJWT(next http.HandlerFunc, deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerTokenFromRequest(r)
		if token == "" {
			writeJSON(w, http.StatusUnauthorized, apiError{Error: "missing bearer token"})
			return
		}

		claims, err := sharedcrypto.ParseToken(token, deps.JWTSecret)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, apiError{Error: "invalid token"})
			return
		}
		if claims.Type != sharedcrypto.TokenTypeAdmin {
			writeJSON(w, http.StatusForbidden, apiError{Error: "admin token required"})
			return
		}

		next(w, r.WithContext(withClaims(r.Context(), claims)))
	}
}
