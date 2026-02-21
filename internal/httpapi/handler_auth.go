package httpapi

import "net/http"

func handleAuthLogin(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	var req bootstrapRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}

	result, err := deps.AuthService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		status, payload := mapLoginError(err)
		writeJSON(w, status, payload)
		return
	}

	writeJSON(w, http.StatusOK, result)
}
