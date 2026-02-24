package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sharedcrypto "github.com/ketsuna-org/sovrabase/internal/shared/crypto"
)

const (
	ScopeAdminsCreate     = "admin.admins.create"
	ScopeUsersList        = "admin.users.list"
	ScopeUsersGet         = "admin.users.get"
	ScopeUsersCreate      = "admin.users.create"
	ScopeUsersUpdate      = "admin.users.update"
	ScopeUsersDelete      = "admin.users.delete"
	ScopeUsersAssignRole  = "admin.users.roles.assign"
	ScopeUsersRemoveRole  = "admin.users.roles.remove"
	ScopeRolesList        = "admin.roles.list"
	ScopeRolesGet         = "admin.roles.get"
	ScopeRolesCreate      = "admin.roles.create"
	ScopeRolesUpdate      = "admin.roles.update"
	ScopeRolesDelete      = "admin.roles.delete"
	ScopeRolesAssignScope = "admin.roles.scopes.assign"
	ScopeRolesRemoveScope = "admin.roles.scopes.remove"
	ScopeScopesList       = "admin.scopes.list"
	ScopeScopesGet        = "admin.scopes.get"
	ScopeScopesCreate     = "admin.scopes.create"
	ScopeScopesUpdate     = "admin.scopes.update"
	ScopeScopesDelete     = "admin.scopes.delete"
)

func (s *service) CreateAdmin(ctx context.Context, actorUserID, email, password string) (User, error) {
	if err := s.Authorize(ctx, actorUserID, ScopeAdminsCreate); err != nil {
		return User{}, err
	}

	actor, err := s.store.GetByID(ctx, strings.TrimSpace(actorUserID))
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return User{}, ErrForbidden
		}
		return User{}, fmt.Errorf("resolve actor: %w", err)
	}
	if !actor.IsRoot {
		return User{}, ErrForbidden
	}

	normalizedEmail, err := normalizeAndValidateEmail(email)
	if err != nil {
		return User{}, err
	}
	if err := validatePassword(password); err != nil {
		return User{}, err
	}

	passwordHash, err := sharedcrypto.HashPassword(password)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.store.CreateUser(ctx, normalizedEmail, passwordHash, UserRoleAdmin, AccountTypeAdmin, false)
	if err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *service) CreateUser(ctx context.Context, input CreateUserInput) (User, error) {
	if err := s.Authorize(ctx, input.ActorUserID, ScopeUsersCreate); err != nil {
		return User{}, err
	}

	normalizedEmail, err := normalizeAndValidateEmail(input.Email)
	if err != nil {
		return User{}, err
	}
	if err := validatePassword(input.Password); err != nil {
		return User{}, err
	}
	if err := validateRole(input.Role); err != nil {
		return User{}, err
	}
	if err := validateAccountType(input.AccountType); err != nil {
		return User{}, err
	}

	passwordHash, err := sharedcrypto.HashPassword(input.Password)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}
	return s.store.CreateUser(ctx, normalizedEmail, passwordHash, input.Role, input.AccountType, false)
}

func (s *service) ListUsers(ctx context.Context, actorUserID string) ([]User, error) {
	if err := s.Authorize(ctx, actorUserID, ScopeUsersList); err != nil {
		return nil, err
	}
	return s.store.ListUsers(ctx)
}

func (s *service) GetUser(ctx context.Context, actorUserID, userID string) (User, error) {
	if err := s.Authorize(ctx, actorUserID, ScopeUsersGet); err != nil {
		return User{}, err
	}
	return s.store.GetByID(ctx, strings.TrimSpace(userID))
}

func (s *service) UpdateUser(ctx context.Context, input UpdateUserInput) (User, error) {
	if err := s.Authorize(ctx, input.ActorUserID, ScopeUsersUpdate); err != nil {
		return User{}, err
	}
	if strings.TrimSpace(input.UserID) == "" {
		return User{}, fmt.Errorf("%w: user id is required", ErrInvalidInput)
	}

	target, err := s.store.GetByID(ctx, strings.TrimSpace(input.UserID))
	if err != nil {
		return User{}, err
	}
	if target.IsRoot {
		return User{}, ErrRootImmutable
	}

	updates := UpdateUserStoreInput{}
	if input.Email != nil {
		normalizedEmail, err := normalizeAndValidateEmail(*input.Email)
		if err != nil {
			return User{}, err
		}
		updates.Email = &normalizedEmail
	}
	if input.Password != nil {
		if err := validatePassword(*input.Password); err != nil {
			return User{}, err
		}
		passwordHash, err := sharedcrypto.HashPassword(*input.Password)
		if err != nil {
			return User{}, fmt.Errorf("hash password: %w", err)
		}
		updates.PasswordHash = &passwordHash
	}
	if input.Role != nil {
		if err := validateRole(*input.Role); err != nil {
			return User{}, err
		}
		updates.Role = input.Role
	}
	if input.AccountType != nil {
		if err := validateAccountType(*input.AccountType); err != nil {
			return User{}, err
		}
		updates.AccountType = input.AccountType
	}
	updates.IsActive = input.IsActive

	return s.store.UpdateUser(ctx, input.UserID, updates)
}

func (s *service) DeleteUser(ctx context.Context, actorUserID, userID string) error {
	if err := s.Authorize(ctx, actorUserID, ScopeUsersDelete); err != nil {
		return err
	}
	target, err := s.store.GetByID(ctx, strings.TrimSpace(userID))
	if err != nil {
		return err
	}
	if target.IsRoot {
		return ErrRootImmutable
	}
	return s.store.DeleteUser(ctx, strings.TrimSpace(userID))
}

func (s *service) CreateRole(ctx context.Context, input CreateRoleInput) (RoleRecord, error) {
	if err := s.Authorize(ctx, input.ActorUserID, ScopeRolesCreate); err != nil {
		return RoleRecord{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return RoleRecord{}, fmt.Errorf("%w: role name is required", ErrInvalidInput)
	}
	if input.ParentRoleID != nil && strings.TrimSpace(*input.ParentRoleID) != "" {
		if _, err := s.store.GetRoleByID(ctx, strings.TrimSpace(*input.ParentRoleID)); err != nil {
			return RoleRecord{}, err
		}
	}
	return s.store.CreateRole(ctx, name, strings.TrimSpace(input.Description), input.ParentRoleID)
}

func (s *service) ListRoles(ctx context.Context, actorUserID string) ([]RoleRecord, error) {
	if err := s.Authorize(ctx, actorUserID, ScopeRolesList); err != nil {
		return nil, err
	}
	return s.store.ListRoles(ctx)
}

func (s *service) GetRole(ctx context.Context, actorUserID, roleID string) (RoleRecord, error) {
	if err := s.Authorize(ctx, actorUserID, ScopeRolesGet); err != nil {
		return RoleRecord{}, err
	}
	return s.store.GetRoleByID(ctx, strings.TrimSpace(roleID))
}

func (s *service) UpdateRole(ctx context.Context, input UpdateRoleInput) (RoleRecord, error) {
	if err := s.Authorize(ctx, input.ActorUserID, ScopeRolesUpdate); err != nil {
		return RoleRecord{}, err
	}
	if input.ParentRoleID != nil {
		parentID := strings.TrimSpace(*input.ParentRoleID)
		if parentID == strings.TrimSpace(input.RoleID) {
			return RoleRecord{}, ErrRoleHierarchyCycle
		}
		if parentID != "" {
			contains, err := s.store.RoleParentContains(ctx, strings.TrimSpace(input.RoleID), parentID)
			if err != nil {
				return RoleRecord{}, err
			}
			if contains {
				return RoleRecord{}, ErrRoleHierarchyCycle
			}
		}
	}
	updates := UpdateRoleStoreInput{
		Name:         input.Name,
		Description:  input.Description,
		ParentRoleID: input.ParentRoleID,
	}
	return s.store.UpdateRole(ctx, strings.TrimSpace(input.RoleID), updates)
}

func (s *service) DeleteRole(ctx context.Context, actorUserID, roleID string) error {
	if err := s.Authorize(ctx, actorUserID, ScopeRolesDelete); err != nil {
		return err
	}
	return s.store.DeleteRole(ctx, strings.TrimSpace(roleID))
}

func (s *service) CreateScope(ctx context.Context, input CreateScopeInput) (ScopeRecord, error) {
	if err := s.Authorize(ctx, input.ActorUserID, ScopeScopesCreate); err != nil {
		return ScopeRecord{}, err
	}
	key := strings.TrimSpace(input.Key)
	if key == "" {
		return ScopeRecord{}, fmt.Errorf("%w: scope key is required", ErrInvalidInput)
	}
	return s.store.CreateScope(ctx, key, strings.TrimSpace(input.Description))
}

func (s *service) ListScopes(ctx context.Context, actorUserID string) ([]ScopeRecord, error) {
	if err := s.Authorize(ctx, actorUserID, ScopeScopesList); err != nil {
		return nil, err
	}
	return s.store.ListScopes(ctx)
}

func (s *service) GetScope(ctx context.Context, actorUserID, scopeID string) (ScopeRecord, error) {
	if err := s.Authorize(ctx, actorUserID, ScopeScopesGet); err != nil {
		return ScopeRecord{}, err
	}
	return s.store.GetScopeByID(ctx, strings.TrimSpace(scopeID))
}

func (s *service) UpdateScope(ctx context.Context, input UpdateScopeInput) (ScopeRecord, error) {
	if err := s.Authorize(ctx, input.ActorUserID, ScopeScopesUpdate); err != nil {
		return ScopeRecord{}, err
	}
	updates := UpdateScopeStoreInput{Key: input.Key, Description: input.Description}
	return s.store.UpdateScope(ctx, strings.TrimSpace(input.ScopeID), updates)
}

func (s *service) DeleteScope(ctx context.Context, actorUserID, scopeID string) error {
	if err := s.Authorize(ctx, actorUserID, ScopeScopesDelete); err != nil {
		return err
	}
	return s.store.DeleteScope(ctx, strings.TrimSpace(scopeID))
}

func (s *service) AssignRoleToUser(ctx context.Context, actorUserID, userID, roleID string) error {
	if err := s.Authorize(ctx, actorUserID, ScopeUsersAssignRole); err != nil {
		return err
	}
	target, err := s.store.GetByID(ctx, strings.TrimSpace(userID))
	if err != nil {
		return err
	}
	if target.IsRoot {
		return ErrRootImmutable
	}
	return s.store.AssignRoleToUser(ctx, strings.TrimSpace(userID), strings.TrimSpace(roleID))
}

func (s *service) RemoveRoleFromUser(ctx context.Context, actorUserID, userID, roleID string) error {
	if err := s.Authorize(ctx, actorUserID, ScopeUsersRemoveRole); err != nil {
		return err
	}
	target, err := s.store.GetByID(ctx, strings.TrimSpace(userID))
	if err != nil {
		return err
	}
	if target.IsRoot {
		return ErrRootImmutable
	}
	return s.store.RemoveRoleFromUser(ctx, strings.TrimSpace(userID), strings.TrimSpace(roleID))
}

func (s *service) AssignScopeToRole(ctx context.Context, actorUserID, roleID, scopeID string) error {
	if err := s.Authorize(ctx, actorUserID, ScopeRolesAssignScope); err != nil {
		return err
	}
	return s.store.AssignScopeToRole(ctx, strings.TrimSpace(roleID), strings.TrimSpace(scopeID))
}

func (s *service) RemoveScopeFromRole(ctx context.Context, actorUserID, roleID, scopeID string) error {
	if err := s.Authorize(ctx, actorUserID, ScopeRolesRemoveScope); err != nil {
		return err
	}
	return s.store.RemoveScopeFromRole(ctx, strings.TrimSpace(roleID), strings.TrimSpace(scopeID))
}

func (s *service) Authorize(ctx context.Context, actorUserID, scope string) error {
	actor, err := s.store.GetByID(ctx, strings.TrimSpace(actorUserID))
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return ErrForbidden
		}
		return fmt.Errorf("resolve actor: %w", err)
	}
	if actor.IsRoot {
		return nil
	}
	if actor.AccountType != AccountTypeAdmin {
		return ErrForbidden
	}
	if strings.TrimSpace(scope) == "" {
		return nil
	}
	scopes, err := s.store.ResolveUserScopes(ctx, actor.ID)
	if err != nil {
		return fmt.Errorf("resolve user scopes: %w", err)
	}
	for _, available := range scopes {
		if available == scope {
			return nil
		}
	}
	return ErrForbidden
}
