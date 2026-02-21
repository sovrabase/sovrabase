package httpapi

import (
	"errors"
	"net/http"

	coreauth "github.com/ketsuna-org/sovrabase/internal/core/auth"
)

func mapBootstrapError(err error) (int, apiError) {
	switch {
	case errors.Is(err, coreauth.ErrInvalidInput):
		return http.StatusBadRequest, apiError{Error: "invalid bootstrap payload"}
	case errors.Is(err, coreauth.ErrBootstrapAlreadyDone):
		return http.StatusConflict, apiError{Error: "bootstrap already completed"}
	default:
		return http.StatusInternalServerError, apiError{Error: "bootstrap failed"}
	}
}

func mapLoginError(err error) (int, apiError) {
	switch {
	case errors.Is(err, coreauth.ErrInvalidInput):
		return http.StatusBadRequest, apiError{Error: "invalid login payload"}
	case errors.Is(err, coreauth.ErrBootstrapRequired):
		return http.StatusConflict, apiError{Error: "bootstrap required before login"}
	case errors.Is(err, coreauth.ErrInvalidCredentials):
		return http.StatusUnauthorized, apiError{Error: "invalid credentials"}
	default:
		return http.StatusInternalServerError, apiError{Error: "login failed"}
	}
}
