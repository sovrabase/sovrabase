package httpapi

import "net/http"

func handleGetConfig(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	required, err := deps.AuthService.GetConfigState(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to resolve bootstrap state"})
		return
	}

	mode := "login"
	message := configLoginMessage
	if required {
		mode = "bootstrap"
		message = configBootstrapMessage
		deps.Logger.Printf("bootstrap still required: use POST /config to create first admin")
	}

	response := configResponse{
		BootstrapRequired: required,
		Message:           message,
		Auth: configAuthSection{
			BootstrapEndpoint: "/config",
			LoginEndpoint:     "/auth/login",
			Mode:              mode,
		},
		Config: sanitizeConfig(deps.Config, deps.EncryptionKeyConfigured, deps.JWTSigningKeyConfigured),
	}
	writeJSON(w, http.StatusOK, response)
}

func handleBootstrapConfig(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	var req bootstrapRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}

	result, err := deps.AuthService.BootstrapFirstAdmin(r.Context(), req.Email, req.Password)
	if err != nil {
		status, payload := mapBootstrapError(err)
		writeJSON(w, status, payload)
		return
	}

	response := bootstrapSuccessResponse{
		Message:     "Admin bootstrap completed.",
		TokenType:   result.TokenType,
		AccessToken: result.AccessToken,
		ExpiresIn:   result.ExpiresIn,
	}
	response.User.ID = result.User.ID
	response.User.Email = result.User.Email
	response.User.Role = result.User.Role

	writeJSON(w, http.StatusCreated, response)
}
