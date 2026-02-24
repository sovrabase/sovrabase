package httpapi

import (
	coreauth "github.com/ketsuna-org/sovrabase/internal/core/auth"
)

type createAdminRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type createUserRequest struct {
	Email       string               `json:"email"`
	Password    string               `json:"password"`
	Role        coreauth.UserRole    `json:"role"`
	AccountType coreauth.AccountType `json:"account_type"`
}

type updateUserRequest struct {
	Email       *string               `json:"email"`
	Password    *string               `json:"password"`
	Role        *coreauth.UserRole    `json:"role"`
	AccountType *coreauth.AccountType `json:"account_type"`
	IsActive    *bool                 `json:"is_active"`
}

type createRoleRequest struct {
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	ParentRoleID *string `json:"parent_role_id"`
}

type updateRoleRequest struct {
	Name         *string `json:"name"`
	Description  *string `json:"description"`
	ParentRoleID *string `json:"parent_role_id"`
}

type createScopeRequest struct {
	Key         string `json:"key"`
	Description string `json:"description"`
}

type updateScopeRequest struct {
	Key         *string `json:"key"`
	Description *string `json:"description"`
}

type userResponse struct {
	ID          string               `json:"id"`
	Email       string               `json:"email"`
	Role        coreauth.UserRole    `json:"role"`
	AccountType coreauth.AccountType `json:"account_type"`
	IsRoot      bool                 `json:"is_root"`
	IsActive    bool                 `json:"is_active"`
}

type roleResponse struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	ParentRoleID *string `json:"parent_role_id"`
}

type scopeResponse struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Description string `json:"description"`
}

func mapUserResponse(user coreauth.User) userResponse {
	return userResponse{
		ID:          user.ID,
		Email:       user.Email,
		Role:        user.Role,
		AccountType: user.AccountType,
		IsRoot:      user.IsRoot,
		IsActive:    user.IsActive,
	}
}

func mapRoleResponse(role coreauth.RoleRecord) roleResponse {
	return roleResponse{ID: role.ID, Name: role.Name, Description: role.Description, ParentRoleID: role.ParentRoleID}
}

func mapScopeResponse(scope coreauth.ScopeRecord) scopeResponse {
	return scopeResponse{ID: scope.ID, Key: scope.Key, Description: scope.Description}
}
