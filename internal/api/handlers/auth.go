package handlers

import (
	"net/http"

	"github.com/ketsuna-org/sovrabase/internal/models/requests"
)

// CreateAuthProviderHandler creates an auth provider for a project
// @Summary Create Auth Provider
// @Tags Auth
// @Security Bearer
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Param request body requests.CreateAuthProviderRequest true "Auth provider data"
// @Success 200
// @Router /project/{id}/auth/providers [post]
func CreateAuthProviderHandler(w http.ResponseWriter, r *http.Request) {
	var req requests.CreateAuthProviderRequest
	_ = req
	// TODO: Implement auth provider creation logic
	w.WriteHeader(http.StatusOK)
}

// ProjectSignupHandler signs up a new user in a project
// @Summary Signup a new User in the project.
// @Tags Auth
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Param request body requests.ProjectSignupRequest true "Signup data"
// @Success 200
// @Router /project/{id}/auth/signup [post]
func ProjectSignupHandler(w http.ResponseWriter, r *http.Request) {
	var req requests.ProjectSignupRequest
	_ = req
	// TODO: Implement project signup logic
	w.WriteHeader(http.StatusOK)
}

// ProjectLoginHandler logs in a user to a project
// @Summary Login a new user in the project !
// @Tags Auth
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Param request body requests.ProjectLoginRequest true "Login data"
// @Success 200
// @Router /project/{id}/auth/login [post]
func ProjectLoginHandler(w http.ResponseWriter, r *http.Request) {
	var req requests.ProjectLoginRequest
	_ = req
	// TODO: Implement project login logic
	w.WriteHeader(http.StatusOK)
}

// ProjectLogoutHandler logs out a user from a project
// @Summary Remove a session of the User !
// @Tags Auth
// @Security Bearer
// @Param id path string true "Project ID"
// @Success 204
// @Router /project/{id}/auth/logout [post]
func ProjectLogoutHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement project logout logic
	w.WriteHeader(http.StatusNoContent)
}

// GetProjectUserHandler gets the current user in a project
// @Summary Get Project User
// @Tags Auth
// @Security Bearer
// @Param id path string true "Project ID"
// @Success 200
// @Router /project/{id}/user [get]
func GetProjectUserHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement get project user logic
	w.WriteHeader(http.StatusOK)
}

// DeleteProjectUserHandler deletes a user from a project
// @Summary Delete Project User
// @Tags Auth
// @Security Bearer
// @Param id path string true "Project ID"
// @Success 200
// @Router /project/{id}/user [delete]
func DeleteProjectUserHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement delete project user logic
	w.WriteHeader(http.StatusOK)
}

// OAuthCallbackHandler handles OAuth callbacks
// @Summary OAuth Callback
// @Tags Auth
// @Accept json
// @Produce json
// @Param id path string true "Project ID"
// @Param provider path string true "Provider Name"
// @Param request body requests.OAuthCallbackRequest true "OAuth callback data"
// @Success 200
// @Router /project/{id}/auth/providers/{provider}/callback [post]
func OAuthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	var req requests.OAuthCallbackRequest
	_ = req
	// TODO: Implement OAuth callback logic
	w.WriteHeader(http.StatusOK)
}
