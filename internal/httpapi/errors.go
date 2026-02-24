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

func mapAdminAPIError(err error) (int, apiError) {
	switch {
	case errors.Is(err, coreauth.ErrInvalidInput):
		return http.StatusBadRequest, apiError{Error: "invalid request payload"}
	case errors.Is(err, coreauth.ErrForbidden):
		return http.StatusForbidden, apiError{Error: "forbidden"}
	case errors.Is(err, coreauth.ErrRootImmutable):
		return http.StatusForbidden, apiError{Error: "root user is immutable"}
	case errors.Is(err, coreauth.ErrRoleHierarchyCycle):
		return http.StatusConflict, apiError{Error: "role hierarchy cycle detected"}
	case errors.Is(err, coreauth.ErrUserNotFound), errors.Is(err, coreauth.ErrRoleNotFound), errors.Is(err, coreauth.ErrScopeNotFound):
		return http.StatusNotFound, apiError{Error: "resource not found"}
	case errors.Is(err, coreauth.ErrConflict):
		return http.StatusConflict, apiError{Error: "resource conflict"}
	default:
		return http.StatusInternalServerError, apiError{Error: "admin api failure"}
	}
}
