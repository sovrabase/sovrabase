package httpapi

type SwaggerAPIError struct {
	Error string `json:"error"`
}

type SwaggerCreateAdminRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type SwaggerCreateUserRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	Role        string `json:"role"`
	AccountType string `json:"account_type"`
}

type SwaggerUpdateUserRequest struct {
	Email       *string `json:"email"`
	Password    *string `json:"password"`
	Role        *string `json:"role"`
	AccountType *string `json:"account_type"`
	IsActive    *bool   `json:"is_active"`
}

type SwaggerUserResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	AccountType string `json:"account_type"`
	IsRoot      bool   `json:"is_root"`
	IsActive    bool   `json:"is_active"`
}

type SwaggerCreateRoleRequest struct {
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	ParentRoleID *string `json:"parent_role_id"`
}

type SwaggerUpdateRoleRequest struct {
	Name         *string `json:"name"`
	Description  *string `json:"description"`
	ParentRoleID *string `json:"parent_role_id"`
}

type SwaggerRoleResponse struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	ParentRoleID *string `json:"parent_role_id"`
}

type SwaggerCreateScopeRequest struct {
	Key         string `json:"key"`
	Description string `json:"description"`
}

type SwaggerUpdateScopeRequest struct {
	Key         *string `json:"key"`
	Description *string `json:"description"`
}

type SwaggerScopeResponse struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Description string `json:"description"`
}

// swaggerAdminRoot godoc
// @Summary Admin API heartbeat
// @Tags admin
// @Security BearerAuth
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/admin/v1 [get]
func swaggerAdminRoot() {}

// swaggerCreateAdmin godoc
// @Summary Create secondary admin (root only)
// @Tags admin
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body SwaggerCreateAdminRequest true "payload"
// @Success 201 {object} SwaggerUserResponse
// @Failure 400 {object} SwaggerAPIError
// @Failure 403 {object} SwaggerAPIError
// @Router /api/admin/v1/admins [post]
func swaggerCreateAdmin() {}

// swaggerListUsers godoc
// @Summary List users
// @Tags admin-users
// @Security BearerAuth
// @Produce json
// @Success 200 {array} SwaggerUserResponse
// @Router /api/admin/v1/users [get]
func swaggerListUsers() {}

// swaggerCreateUser godoc
// @Summary Create user
// @Tags admin-users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body SwaggerCreateUserRequest true "payload"
// @Success 201 {object} SwaggerUserResponse
// @Failure 400 {object} SwaggerAPIError
// @Failure 409 {object} SwaggerAPIError
// @Router /api/admin/v1/users [post]
func swaggerCreateUser() {}

// swaggerGetUser godoc
// @Summary Get user
// @Tags admin-users
// @Security BearerAuth
// @Produce json
// @Param user_id path string true "User ID"
// @Success 200 {object} SwaggerUserResponse
// @Failure 404 {object} SwaggerAPIError
// @Router /api/admin/v1/users/{user_id} [get]
func swaggerGetUser() {}

// swaggerUpdateUser godoc
// @Summary Update user
// @Tags admin-users
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param user_id path string true "User ID"
// @Param body body SwaggerUpdateUserRequest true "payload"
// @Success 200 {object} SwaggerUserResponse
// @Failure 403 {object} SwaggerAPIError
// @Failure 404 {object} SwaggerAPIError
// @Router /api/admin/v1/users/{user_id} [put]
func swaggerUpdateUser() {}

// swaggerDeleteUser godoc
// @Summary Delete user
// @Tags admin-users
// @Security BearerAuth
// @Param user_id path string true "User ID"
// @Success 204
// @Failure 403 {object} SwaggerAPIError
// @Failure 404 {object} SwaggerAPIError
// @Router /api/admin/v1/users/{user_id} [delete]
func swaggerDeleteUser() {}

// swaggerAssignRoleToUser godoc
// @Summary Assign role to user
// @Tags admin-users
// @Security BearerAuth
// @Param user_id path string true "User ID"
// @Param role_id path string true "Role ID"
// @Success 204
// @Failure 403 {object} SwaggerAPIError
// @Failure 404 {object} SwaggerAPIError
// @Router /api/admin/v1/users/{user_id}/roles/{role_id} [post]
func swaggerAssignRoleToUser() {}

// swaggerRemoveRoleFromUser godoc
// @Summary Remove role from user
// @Tags admin-users
// @Security BearerAuth
// @Param user_id path string true "User ID"
// @Param role_id path string true "Role ID"
// @Success 204
// @Failure 403 {object} SwaggerAPIError
// @Failure 404 {object} SwaggerAPIError
// @Router /api/admin/v1/users/{user_id}/roles/{role_id} [delete]
func swaggerRemoveRoleFromUser() {}

// swaggerListRoles godoc
// @Summary List roles
// @Tags admin-roles
// @Security BearerAuth
// @Produce json
// @Success 200 {array} SwaggerRoleResponse
// @Router /api/admin/v1/roles [get]
func swaggerListRoles() {}

// swaggerCreateRole godoc
// @Summary Create role
// @Tags admin-roles
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body SwaggerCreateRoleRequest true "payload"
// @Success 201 {object} SwaggerRoleResponse
// @Failure 400 {object} SwaggerAPIError
// @Failure 404 {object} SwaggerAPIError
// @Router /api/admin/v1/roles [post]
func swaggerCreateRole() {}

// swaggerGetRole godoc
// @Summary Get role
// @Tags admin-roles
// @Security BearerAuth
// @Produce json
// @Param role_id path string true "Role ID"
// @Success 200 {object} SwaggerRoleResponse
// @Failure 404 {object} SwaggerAPIError
// @Router /api/admin/v1/roles/{role_id} [get]
func swaggerGetRole() {}

// swaggerUpdateRole godoc
// @Summary Update role
// @Tags admin-roles
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param role_id path string true "Role ID"
// @Param body body SwaggerUpdateRoleRequest true "payload"
// @Success 200 {object} SwaggerRoleResponse
// @Failure 404 {object} SwaggerAPIError
// @Failure 409 {object} SwaggerAPIError
// @Router /api/admin/v1/roles/{role_id} [put]
func swaggerUpdateRole() {}

// swaggerDeleteRole godoc
// @Summary Delete role
// @Tags admin-roles
// @Security BearerAuth
// @Param role_id path string true "Role ID"
// @Success 204
// @Failure 404 {object} SwaggerAPIError
// @Router /api/admin/v1/roles/{role_id} [delete]
func swaggerDeleteRole() {}

// swaggerAssignScopeToRole godoc
// @Summary Assign scope to role
// @Tags admin-roles
// @Security BearerAuth
// @Param role_id path string true "Role ID"
// @Param scope_id path string true "Scope ID"
// @Success 204
// @Failure 404 {object} SwaggerAPIError
// @Router /api/admin/v1/roles/{role_id}/scopes/{scope_id} [post]
func swaggerAssignScopeToRole() {}

// swaggerRemoveScopeFromRole godoc
// @Summary Remove scope from role
// @Tags admin-roles
// @Security BearerAuth
// @Param role_id path string true "Role ID"
// @Param scope_id path string true "Scope ID"
// @Success 204
// @Failure 404 {object} SwaggerAPIError
// @Router /api/admin/v1/roles/{role_id}/scopes/{scope_id} [delete]
func swaggerRemoveScopeFromRole() {}

// swaggerListScopes godoc
// @Summary List scopes
// @Tags admin-scopes
// @Security BearerAuth
// @Produce json
// @Success 200 {array} SwaggerScopeResponse
// @Router /api/admin/v1/scopes [get]
func swaggerListScopes() {}

// swaggerCreateScope godoc
// @Summary Create scope
// @Tags admin-scopes
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body SwaggerCreateScopeRequest true "payload"
// @Success 201 {object} SwaggerScopeResponse
// @Failure 400 {object} SwaggerAPIError
// @Failure 409 {object} SwaggerAPIError
// @Router /api/admin/v1/scopes [post]
func swaggerCreateScope() {}

// swaggerGetScope godoc
// @Summary Get scope
// @Tags admin-scopes
// @Security BearerAuth
// @Produce json
// @Param scope_id path string true "Scope ID"
// @Success 200 {object} SwaggerScopeResponse
// @Failure 404 {object} SwaggerAPIError
// @Router /api/admin/v1/scopes/{scope_id} [get]
func swaggerGetScope() {}

// swaggerUpdateScope godoc
// @Summary Update scope
// @Tags admin-scopes
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param scope_id path string true "Scope ID"
// @Param body body SwaggerUpdateScopeRequest true "payload"
// @Success 200 {object} SwaggerScopeResponse
// @Failure 404 {object} SwaggerAPIError
// @Router /api/admin/v1/scopes/{scope_id} [put]
func swaggerUpdateScope() {}

// swaggerDeleteScope godoc
// @Summary Delete scope
// @Tags admin-scopes
// @Security BearerAuth
// @Param scope_id path string true "Scope ID"
// @Success 204
// @Failure 404 {object} SwaggerAPIError
// @Router /api/admin/v1/scopes/{scope_id} [delete]
func swaggerDeleteScope() {}
