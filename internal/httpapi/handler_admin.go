package httpapi

import (
	"net/http"
	"strings"

	coreauth "github.com/ketsuna-org/sovrabase/internal/core/auth"
)

const adminPrefix = "/api/admin/v1"

func handleAdminAPI(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	claims, ok := claimsFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, apiError{Error: "missing auth context"})
		return
	}
	actorUserID := claims.UserID

	path := strings.TrimPrefix(r.URL.Path, adminPrefix)
	path = strings.Trim(path, "/")
	segments := []string{}
	if path != "" {
		segments = strings.Split(path, "/")
	}

	if len(segments) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"service": "admin", "version": "v1"})
		return
	}

	switch segments[0] {
	case "admins":
		handleAdminsResource(w, r, deps, actorUserID, segments)
	case "users":
		handleUsersResource(w, r, deps, actorUserID, segments)
	case "roles":
		handleRolesResource(w, r, deps, actorUserID, segments)
	case "scopes":
		handleScopesResource(w, r, deps, actorUserID, segments)
	default:
		writeJSON(w, http.StatusNotFound, apiError{Error: "resource not found"})
	}
}

func handleAdminsResource(w http.ResponseWriter, r *http.Request, deps Dependencies, actorUserID string, segments []string) {
	if len(segments) != 1 || r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var req createAdminRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}
	created, err := deps.AuthService.CreateAdmin(r.Context(), actorUserID, req.Email, req.Password)
	if err != nil {
		status, payload := mapAdminAPIError(err)
		writeJSON(w, status, payload)
		return
	}
	writeJSON(w, http.StatusCreated, mapUserResponse(created))
}

func handleUsersResource(w http.ResponseWriter, r *http.Request, deps Dependencies, actorUserID string, segments []string) {
	if len(segments) == 1 {
		switch r.Method {
		case http.MethodGet:
			users, err := deps.AuthService.ListUsers(r.Context(), actorUserID)
			if err != nil {
				status, payload := mapAdminAPIError(err)
				writeJSON(w, status, payload)
				return
			}
			out := make([]userResponse, 0, len(users))
			for _, user := range users {
				out = append(out, mapUserResponse(user))
			}
			writeJSON(w, http.StatusOK, out)
		case http.MethodPost:
			var req createUserRequest
			if err := decodeJSONBody(r, &req); err != nil {
				writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
				return
			}
			created, err := deps.AuthService.CreateUser(r.Context(), coreauth.CreateUserInput{
				ActorUserID: actorUserID,
				Email:       req.Email,
				Password:    req.Password,
				Role:        req.Role,
				AccountType: req.AccountType,
			})
			if err != nil {
				status, payload := mapAdminAPIError(err)
				writeJSON(w, status, payload)
				return
			}
			writeJSON(w, http.StatusCreated, mapUserResponse(created))
		default:
			writeMethodNotAllowed(w)
		}
		return
	}

	if len(segments) == 2 {
		userID := segments[1]
		switch r.Method {
		case http.MethodGet:
			user, err := deps.AuthService.GetUser(r.Context(), actorUserID, userID)
			if err != nil {
				status, payload := mapAdminAPIError(err)
				writeJSON(w, status, payload)
				return
			}
			writeJSON(w, http.StatusOK, mapUserResponse(user))
		case http.MethodPut:
			var req updateUserRequest
			if err := decodeJSONBody(r, &req); err != nil {
				writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
				return
			}
			updated, err := deps.AuthService.UpdateUser(r.Context(), coreauth.UpdateUserInput{
				ActorUserID: actorUserID,
				UserID:      userID,
				Email:       req.Email,
				Password:    req.Password,
				Role:        req.Role,
				AccountType: req.AccountType,
				IsActive:    req.IsActive,
			})
			if err != nil {
				status, payload := mapAdminAPIError(err)
				writeJSON(w, status, payload)
				return
			}
			writeJSON(w, http.StatusOK, mapUserResponse(updated))
		case http.MethodDelete:
			err := deps.AuthService.DeleteUser(r.Context(), actorUserID, userID)
			if err != nil {
				status, payload := mapAdminAPIError(err)
				writeJSON(w, status, payload)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			writeMethodNotAllowed(w)
		}
		return
	}

	if len(segments) == 4 && segments[2] == "roles" {
		userID := segments[1]
		roleID := segments[3]
		switch r.Method {
		case http.MethodPost:
			err := deps.AuthService.AssignRoleToUser(r.Context(), actorUserID, userID, roleID)
			if err != nil {
				status, payload := mapAdminAPIError(err)
				writeJSON(w, status, payload)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case http.MethodDelete:
			err := deps.AuthService.RemoveRoleFromUser(r.Context(), actorUserID, userID, roleID)
			if err != nil {
				status, payload := mapAdminAPIError(err)
				writeJSON(w, status, payload)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			writeMethodNotAllowed(w)
		}
		return
	}

	writeJSON(w, http.StatusNotFound, apiError{Error: "resource not found"})
}

func handleRolesResource(w http.ResponseWriter, r *http.Request, deps Dependencies, actorUserID string, segments []string) {
	if len(segments) == 1 {
		switch r.Method {
		case http.MethodGet:
			roles, err := deps.AuthService.ListRoles(r.Context(), actorUserID)
			if err != nil {
				status, payload := mapAdminAPIError(err)
				writeJSON(w, status, payload)
				return
			}
			out := make([]roleResponse, 0, len(roles))
			for _, role := range roles {
				out = append(out, mapRoleResponse(role))
			}
			writeJSON(w, http.StatusOK, out)
		case http.MethodPost:
			var req createRoleRequest
			if err := decodeJSONBody(r, &req); err != nil {
				writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
				return
			}
			created, err := deps.AuthService.CreateRole(r.Context(), coreauth.CreateRoleInput{
				ActorUserID:  actorUserID,
				Name:         req.Name,
				Description:  req.Description,
				ParentRoleID: req.ParentRoleID,
			})
			if err != nil {
				status, payload := mapAdminAPIError(err)
				writeJSON(w, status, payload)
				return
			}
			writeJSON(w, http.StatusCreated, mapRoleResponse(created))
		default:
			writeMethodNotAllowed(w)
		}
		return
	}

	if len(segments) == 2 {
		roleID := segments[1]
		switch r.Method {
		case http.MethodGet:
			role, err := deps.AuthService.GetRole(r.Context(), actorUserID, roleID)
			if err != nil {
				status, payload := mapAdminAPIError(err)
				writeJSON(w, status, payload)
				return
			}
			writeJSON(w, http.StatusOK, mapRoleResponse(role))
		case http.MethodPut:
			var req updateRoleRequest
			if err := decodeJSONBody(r, &req); err != nil {
				writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
				return
			}
			updated, err := deps.AuthService.UpdateRole(r.Context(), coreauth.UpdateRoleInput{
				ActorUserID:  actorUserID,
				RoleID:       roleID,
				Name:         req.Name,
				Description:  req.Description,
				ParentRoleID: req.ParentRoleID,
			})
			if err != nil {
				status, payload := mapAdminAPIError(err)
				writeJSON(w, status, payload)
				return
			}
			writeJSON(w, http.StatusOK, mapRoleResponse(updated))
		case http.MethodDelete:
			err := deps.AuthService.DeleteRole(r.Context(), actorUserID, roleID)
			if err != nil {
				status, payload := mapAdminAPIError(err)
				writeJSON(w, status, payload)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			writeMethodNotAllowed(w)
		}
		return
	}

	if len(segments) == 4 && segments[2] == "scopes" {
		roleID := segments[1]
		scopeID := segments[3]
		switch r.Method {
		case http.MethodPost:
			err := deps.AuthService.AssignScopeToRole(r.Context(), actorUserID, roleID, scopeID)
			if err != nil {
				status, payload := mapAdminAPIError(err)
				writeJSON(w, status, payload)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case http.MethodDelete:
			err := deps.AuthService.RemoveScopeFromRole(r.Context(), actorUserID, roleID, scopeID)
			if err != nil {
				status, payload := mapAdminAPIError(err)
				writeJSON(w, status, payload)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			writeMethodNotAllowed(w)
		}
		return
	}

	writeJSON(w, http.StatusNotFound, apiError{Error: "resource not found"})
}

func handleScopesResource(w http.ResponseWriter, r *http.Request, deps Dependencies, actorUserID string, segments []string) {
	if len(segments) == 1 {
		switch r.Method {
		case http.MethodGet:
			scopes, err := deps.AuthService.ListScopes(r.Context(), actorUserID)
			if err != nil {
				status, payload := mapAdminAPIError(err)
				writeJSON(w, status, payload)
				return
			}
			out := make([]scopeResponse, 0, len(scopes))
			for _, scope := range scopes {
				out = append(out, mapScopeResponse(scope))
			}
			writeJSON(w, http.StatusOK, out)
		case http.MethodPost:
			var req createScopeRequest
			if err := decodeJSONBody(r, &req); err != nil {
				writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
				return
			}
			created, err := deps.AuthService.CreateScope(r.Context(), coreauth.CreateScopeInput{
				ActorUserID: actorUserID,
				Key:         req.Key,
				Description: req.Description,
			})
			if err != nil {
				status, payload := mapAdminAPIError(err)
				writeJSON(w, status, payload)
				return
			}
			writeJSON(w, http.StatusCreated, mapScopeResponse(created))
		default:
			writeMethodNotAllowed(w)
		}
		return
	}

	if len(segments) == 2 {
		scopeID := segments[1]
		switch r.Method {
		case http.MethodGet:
			scope, err := deps.AuthService.GetScope(r.Context(), actorUserID, scopeID)
			if err != nil {
				status, payload := mapAdminAPIError(err)
				writeJSON(w, status, payload)
				return
			}
			writeJSON(w, http.StatusOK, mapScopeResponse(scope))
		case http.MethodPut:
			var req updateScopeRequest
			if err := decodeJSONBody(r, &req); err != nil {
				writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
				return
			}
			updated, err := deps.AuthService.UpdateScope(r.Context(), coreauth.UpdateScopeInput{
				ActorUserID: actorUserID,
				ScopeID:     scopeID,
				Key:         req.Key,
				Description: req.Description,
			})
			if err != nil {
				status, payload := mapAdminAPIError(err)
				writeJSON(w, status, payload)
				return
			}
			writeJSON(w, http.StatusOK, mapScopeResponse(updated))
		case http.MethodDelete:
			err := deps.AuthService.DeleteScope(r.Context(), actorUserID, scopeID)
			if err != nil {
				status, payload := mapAdminAPIError(err)
				writeJSON(w, status, payload)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			writeMethodNotAllowed(w)
		}
		return
	}

	writeJSON(w, http.StatusNotFound, apiError{Error: "resource not found"})
}
