package handlers

import (
	"net/http"

	"github.com/ketsuna-org/sovrabase/internal/models/requests"
)

// GetAllUsersHandler gets all registered users
// @Summary Get all registered Users !
// @Tags Admin
// @Security Bearer
// @Success 200 {array} responses.User
// @Router /admin/users [get]
func GetAllUsersHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement get all users logic
	w.WriteHeader(http.StatusOK)
}

// CreateUserHandler creates a new user
// @Summary Create an User (Need to be authentified)
// @Tags Admin
// @Security Bearer
// @Accept json
// @Produce json
// @Param request body requests.CreateUserRequest true "User creation data"
// @Success 200 {object} responses.CreateUserResponse
// @Router /admin/create [post]
func CreateUserHandler(w http.ResponseWriter, r *http.Request) {
	var req requests.CreateUserRequest
	_ = req
	// TODO: Implement create user logic
	w.WriteHeader(http.StatusOK)
}

// GetAllProjectsHandler gets all projects
// @Summary Get a list of all projects created on the Server
// @Tags Admin
// @Security Bearer
// @Success 200 {array} responses.Project
// @Router /admin/projects [get]
func GetAllProjectsHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement get all projects logic
	w.WriteHeader(http.StatusOK)
}

// GetAllOrganizationsHandler gets all organizations
// @Summary Return the list of organisation created on the server
// @Tags Admin
// @Security Bearer
// @Success 200 {array} responses.Organisation
// @Router /admin/organizations [get]
func GetAllOrganizationsHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement get all organizations logic
	w.WriteHeader(http.StatusOK)
}

// GetAdminMetricsHandler gets overall server metrics
// @Summary Get overall metric usage of the server
// @Tags Admin
// @Security Bearer
// @Success 200 {array} responses.AdminMetric
// @Router /admin/metrics [get]
func GetAdminMetricsHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement get admin metrics logic
	w.WriteHeader(http.StatusOK)
}
