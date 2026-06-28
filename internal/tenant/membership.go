// Package tenant implements multi-tenant project isolation for Sovrabase.
package tenant

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cockroachdb/pebble"
)

// Role represents a project-level permission level.
type Role string

const (
	RoleOwner       Role = "owner"
	RoleAdmin       Role = "admin"
	RoleDeveloper   Role = "developer"
	RoleViewer      Role = "viewer"
)

// TeamMember represents a user's membership in a project.
type TeamMember struct {
	UserID    string    `json:"user_id"`
	ProjectID string    `json:"project_id"`
	Role      Role      `json:"role"`
	JoinedAt  time.Time `json:"joined_at"`
	IsOwner   bool      `json:"is_owner"`
}

// InvitationStatus describes the current state of an invitation.
type InvitationStatus string

const (
	InvitationPending  InvitationStatus = "pending"
	InvitationAccepted InvitationStatus = "accepted"
	InvitationExpired  InvitationStatus = "expired"
	InvitationRevoked  InvitationStatus = "revoked"
)

// Invitation represents a pending invitation for a user to join a project.
type Invitation struct {
	Token      string            `json:"token"`
	ProjectID  string            `json:"project_id"`
	Email      string            `json:"email"`
	Role       Role              `json:"role"`
	Status     InvitationStatus  `json:"status"`
	CreatedBy  string            `json:"created_by"`
	CreatedAt  time.Time         `json:"created_at"`
	ExpiresAt  time.Time         `json:"expires_at"`
	AcceptedAt *time.Time        `json:"accepted_at,omitempty"`
}

// TeamStore manages project team membership and invitations, backed by a Pebble
// database (typically the master database from ProjectManager).
type TeamStore struct {
	db *pebble.DB
}

// NewTeamStore creates a TeamStore backed by the given Pebble database.
func NewTeamStore(db *pebble.DB) *TeamStore {
	return &TeamStore{db: db}
}

// key helpers

func teamMemberKey(projectID, userID string) []byte {
	return []byte("team:" + projectID + ":" + userID)
}

func inviteKey(token string) []byte {
	return []byte("invite:" + token)
}

func inviteByProjectKey(projectID, token string) []byte {
	return []byte("invite_by_project:" + projectID + ":" + token)
}

func teamPrefix(projectID string) []byte {
	return []byte("team:" + projectID + ":")
}

func inviteByProjectPrefix(projectID string) []byte {
	return []byte("invite_by_project:" + projectID + ":")
}

// AddMember adds a user to a project team. If the user is already a member,
// returns an error.
func (ts *TeamStore) AddMember(projectID, userID string, role Role) error {
	key := teamMemberKey(projectID, userID)

	// Check for duplicate
	_, closer, err := ts.db.Get(key)
	if err != nil && err != pebble.ErrNotFound {
		return fmt.Errorf("team: check membership: %w", err)
	}
	if closer != nil {
		closer.Close()
		return fmt.Errorf("team: user %s is already a member of project %s", userID, projectID)
	}

	member := TeamMember{
		UserID:    userID,
		ProjectID: projectID,
		Role:      role,
		JoinedAt:  time.Now().UTC(),
		IsOwner:   role == RoleOwner,
	}

	data, err := json.Marshal(member)
	if err != nil {
		return fmt.Errorf("team: marshal member: %w", err)
	}

	// Write the member record
	if err := ts.db.Set(key, data, pebble.Sync); err != nil {
		return fmt.Errorf("team: save member: %w", err)
	}

	// Update the member-project index for efficient lookup
	if err := ts.AddMemberProjectIndex(userID, projectID); err != nil {
		// Rollback member record if index update fails
		ts.db.Delete(key, pebble.Sync)
		return fmt.Errorf("team: update member-project index: %w", err)
	}

	return nil
}

// RemoveMember removes a user from a project team.
func (ts *TeamStore) RemoveMember(projectID, userID string) error {
	key := teamMemberKey(projectID, userID)

	_, closer, err := ts.db.Get(key)
	if err == pebble.ErrNotFound {
		return fmt.Errorf("team: user %s is not a member of project %s", userID, projectID)
	}
	if err != nil {
		return fmt.Errorf("team: check membership: %w", err)
	}
	closer.Close()

	// Delete the member record
	if err := ts.db.Delete(key, pebble.Sync); err != nil {
		return fmt.Errorf("team: delete member: %w", err)
	}

	// Remove from the member-project index
	if err := ts.RemoveMemberProjectIndex(userID, projectID); err != nil {
		return fmt.Errorf("team: remove member-project index: %w", err)
	}

	return nil
}

// ListMembers returns all members of a project.
func (ts *TeamStore) ListMembers(projectID string) ([]TeamMember, error) {
	prefix := teamPrefix(projectID)
	iter, err := ts.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: keyUpperBound(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("team: list members iter: %w", err)
	}
	defer iter.Close()

	var members []TeamMember
	for iter.First(); iter.Valid(); iter.Next() {
		var m TeamMember
		if err := json.Unmarshal(iter.Value(), &m); err != nil {
			continue
		}
		members = append(members, m)
	}
	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("team: iterate members: %w", err)
	}
	return members, nil
}

// UpdateMemberRole changes the role of an existing team member.
func (ts *TeamStore) UpdateMemberRole(projectID, userID string, newRole Role) error {
	key := teamMemberKey(projectID, userID)

	val, closer, err := ts.db.Get(key)
	if err == pebble.ErrNotFound {
		return fmt.Errorf("team: user %s is not a member of project %s", userID, projectID)
	}
	if err != nil {
		return fmt.Errorf("team: get member: %w", err)
	}
	defer closer.Close()

	var member TeamMember
	if err := json.Unmarshal(val, &member); err != nil {
		return fmt.Errorf("team: unmarshal member: %w", err)
	}

	member.Role = newRole
	member.IsOwner = newRole == RoleOwner

	data, err := json.Marshal(member)
	if err != nil {
		return fmt.Errorf("team: marshal member: %w", err)
	}

	return ts.db.Set(key, data, pebble.Sync)
}

// GetMember retrieves a specific team member by project and user ID.
func (ts *TeamStore) GetMember(projectID, userID string) (*TeamMember, error) {
	key := teamMemberKey(projectID, userID)

	val, closer, err := ts.db.Get(key)
	if err == pebble.ErrNotFound {
		return nil, fmt.Errorf("team: user %s is not a member of project %s", userID, projectID)
	}
	if err != nil {
		return nil, fmt.Errorf("team: get member: %w", err)
	}
	defer closer.Close()

	var member TeamMember
	if err := json.Unmarshal(val, &member); err != nil {
		return nil, fmt.Errorf("team: unmarshal member: %w", err)
	}
	return &member, nil
}

// IsMember returns true if the user is a member of the project.
func (ts *TeamStore) IsMember(projectID, userID string) bool {
	key := teamMemberKey(projectID, userID)
	_, closer, err := ts.db.Get(key)
	if err != nil {
		return false
	}
	closer.Close()
	return true
}

// HasRole returns true if the user is a member of the project and has one of
// the given roles.
func (ts *TeamStore) HasRole(projectID, userID string, allowedRoles ...Role) bool {
	key := teamMemberKey(projectID, userID)
	val, closer, err := ts.db.Get(key)
	if err != nil {
		return false
	}
	defer closer.Close()

	var member TeamMember
	if err := json.Unmarshal(val, &member); err != nil {
		return false
	}

	for _, r := range allowedRoles {
		if member.Role == r {
			return true
		}
	}
	return false
}

// generateInviteToken creates a cryptographically random hex token for
// invitations.
func generateInviteToken() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("team: generate invite token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// CreateInvitation creates a new invitation for a user to join a project.
func (ts *TeamStore) CreateInvitation(projectID, email, createdBy string, role Role, ttl time.Duration) (*Invitation, error) {
	token, err := generateInviteToken()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	inv := &Invitation{
		Token:     token,
		ProjectID: projectID,
		Email:     email,
		Role:      role,
		Status:    InvitationPending,
		CreatedBy: createdBy,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}

	data, err := json.Marshal(inv)
	if err != nil {
		return nil, fmt.Errorf("team: marshal invitation: %w", err)
	}

	// Write both the primary key and the by-project index
	if err := ts.db.Set(inviteKey(token), data, pebble.Sync); err != nil {
		return nil, fmt.Errorf("team: save invitation: %w", err)
	}
	if err := ts.db.Set(inviteByProjectKey(projectID, token), data, pebble.Sync); err != nil {
		return nil, fmt.Errorf("team: save invitation index: %w", err)
	}

	return inv, nil
}

// GetInvitation retrieves an invitation by its token.
func (ts *TeamStore) GetInvitation(token string) (*Invitation, error) {
	val, closer, err := ts.db.Get(inviteKey(token))
	if err == pebble.ErrNotFound {
		return nil, fmt.Errorf("team: invitation %q not found", token)
	}
	if err != nil {
		return nil, fmt.Errorf("team: get invitation: %w", err)
	}
	defer closer.Close()

	var inv Invitation
	if err := json.Unmarshal(val, &inv); err != nil {
		return nil, fmt.Errorf("team: unmarshal invitation: %w", err)
	}
	return &inv, nil
}

// AcceptInvitation accepts a pending invitation and adds the user as a team
// member. Returns the created TeamMember. If the invitation is not pending,
// already expired, or already revoked, an error is returned.
func (ts *TeamStore) AcceptInvitation(token, userID string) (*TeamMember, error) {
	inv, err := ts.GetInvitation(token)
	if err != nil {
		return nil, err
	}

	if inv.Status != InvitationPending {
		return nil, fmt.Errorf("team: invitation %q is %s (not pending)", token, inv.Status)
	}

	if time.Now().UTC().After(inv.ExpiresAt) {
		// Mark as expired
		inv.Status = InvitationExpired
		_ = ts.saveInvitation(inv)
		return nil, fmt.Errorf("team: invitation %q has expired", token)
	}

	// Add the user as a team member
	if err := ts.AddMember(inv.ProjectID, userID, inv.Role); err != nil {
		return nil, err
	}

	// Mark invitation as accepted
	now := time.Now().UTC()
	inv.Status = InvitationAccepted
	inv.AcceptedAt = &now
	if err := ts.saveInvitation(inv); err != nil {
		return nil, err
	}

	member := &TeamMember{
		UserID:    userID,
		ProjectID: inv.ProjectID,
		Role:      inv.Role,
		JoinedAt:  now,
		IsOwner:   inv.Role == RoleOwner,
	}
	return member, nil
}

// RevokeInvitation marks a pending invitation as revoked.
func (ts *TeamStore) RevokeInvitation(token string) error {
	inv, err := ts.GetInvitation(token)
	if err != nil {
		return err
	}

	if inv.Status != InvitationPending {
		return fmt.Errorf("team: invitation %q is %s (not pending)", token, inv.Status)
	}

	inv.Status = InvitationRevoked
	return ts.saveInvitation(inv)
}

// ListInvitations returns all invitations for a project.
func (ts *TeamStore) ListInvitations(projectID string) ([]Invitation, error) {
	prefix := inviteByProjectPrefix(projectID)
	iter, err := ts.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: keyUpperBound(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("team: list invitations iter: %w", err)
	}
	defer iter.Close()

	var invitations []Invitation
	for iter.First(); iter.Valid(); iter.Next() {
		var inv Invitation
		if err := json.Unmarshal(iter.Value(), &inv); err != nil {
			continue
		}
		invitations = append(invitations, inv)
	}
	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("team: iterate invitations: %w", err)
	}
	return invitations, nil
}

// saveInvitation persists an invitation to both the primary key and the
// by-project index.
func (ts *TeamStore) saveInvitation(inv *Invitation) error {
	data, err := json.Marshal(inv)
	if err != nil {
		return fmt.Errorf("team: marshal invitation: %w", err)
	}
	if err := ts.db.Set(inviteKey(inv.Token), data, pebble.Sync); err != nil {
		return fmt.Errorf("team: save invitation: %w", err)
	}
	if err := ts.db.Set(inviteByProjectKey(inv.ProjectID, inv.Token), data, pebble.Sync); err != nil {
		return fmt.Errorf("team: save invitation index: %w", err)
	}
	return nil
}

// ─── Member Credentials Store ────────────────────────────────────────────────
// Simple global store for team member login credentials so they can access
// the dashboard. Keyed by email (not per-project).

const memberCredPrefix = "mcred:"
const userEmailIdxPrefix = "uidx:"

func memberCredKey(email string) []byte {
	return []byte(memberCredPrefix + email)
}

func userEmailIdxKey(userID string) []byte {
	return []byte(userEmailIdxPrefix + userID)
}

// StoreMemberEmail stores just the email lookup index for a user, without
// creating a member credential (used for admin/owner email resolution).
func (ts *TeamStore) StoreMemberEmail(userID, email string) error {
	return ts.db.Set(userEmailIdxKey(userID), []byte(email), pebble.Sync)
}

// StoreMemberCredential saves a password hash and userID for a team member email.
// Also maintains a reverse index (uidx:{userID}) for O(1) email lookups.
func (ts *TeamStore) StoreMemberCredential(email, userID, passwordHash string) error {
	data := userID + "\n" + passwordHash
	if err := ts.db.Set(memberCredKey(email), []byte(data), pebble.Sync); err != nil {
		return err
	}
	// Maintain reverse index
	return ts.db.Set(userEmailIdxKey(userID), []byte(email), pebble.Sync)
}

// GetMemberCredential returns the stored password hash and userID for an email,
// or an error if not found.
func (ts *TeamStore) GetMemberCredential(email string) (userID, passwordHash string, err error) {
	val, closer, err := ts.db.Get(memberCredKey(email))
	if err != nil {
		return "", "", err
	}
	defer closer.Close()
	parts := strings.SplitN(string(val), "\n", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid credential format")
	}
	return parts[0], parts[1], nil
}

// ListAllMembers returns all member credentials (email and userID only, password excluded)
// by iterating over the mcred:* prefix. Also includes project_ids for each member.
func (ts *TeamStore) ListAllMembers() ([]map[string]interface{}, error) {
	prefix := []byte(memberCredPrefix)
	iter, err := ts.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: keyUpperBound(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("team: list all members iter: %w", err)
	}
	defer iter.Close()

	var members []map[string]interface{}
	for iter.First(); iter.Valid(); iter.Next() {
		// Key format: mcred:{email}
		email := strings.TrimPrefix(string(iter.Key()), memberCredPrefix)
		if email == "" {
			continue
		}

		val := iter.Value()
		parts := strings.SplitN(string(val), "\n", 2)
		if len(parts) >= 1 && parts[0] != "" {
			userID := parts[0]
			// Get member projects efficiently using index
			projectIDs, _ := ts.GetMemberProjects(userID) // ignore errors, may have no projects
			members = append(members, map[string]interface{}{
				"user_id":      userID,
				"email":        email,
				"project_ids":  projectIDs,
				"project_count": len(projectIDs),
			})
		}
	}
	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("team: iterate all members: %w", err)
	}
	if members == nil {
		members = []map[string]interface{}{}
	}
	return members, nil
}

// DeleteMember removes the member credential entry for the given email
// AND removes the member from all projects they belong to.
func (ts *TeamStore) DeleteMember(email string) error {
	// First, get the userID from the credential
	val, closer, err := ts.db.Get(memberCredKey(email))
	if err != nil && err != pebble.ErrNotFound {
		return fmt.Errorf("get credential: %w", err)
	}
	var userID string
	if err == nil {
		defer closer.Close()
		parts := strings.SplitN(string(val), "\n", 2)
		if len(parts) >= 1 && parts[0] != "" {
			userID = parts[0]
		}
	}

	// Delete the credential and index
	if err := ts.db.Delete(memberCredKey(email), pebble.Sync); err != nil && err != pebble.ErrNotFound {
		return fmt.Errorf("delete member credential: %w", err)
	}

	// If we have a userID, remove index and all project memberships
	if userID != "" {
		_ = ts.db.Delete(userEmailIdxKey(userID), pebble.Sync) // ignore if not found

		projectIDs, err := ts.GetMemberProjects(userID)
		if err != nil {
			return fmt.Errorf("get member projects: %w", err)
		}
		for _, projectID := range projectIDs {
			_ = ts.RemoveMember(projectID, userID) // ignore errors for individual removals
		}
	}

	return nil
}

// GetUserIDByEmail retrieves the user ID for a given email from credentials.
func (ts *TeamStore) GetUserIDByEmail(email string) (string, error) {
	val, closer, err := ts.db.Get(memberCredKey(email))
	if err != nil {
		if err == pebble.ErrNotFound {
			return "", fmt.Errorf("user not found")
		}
		return "", fmt.Errorf("get user ID: %w", err)
	}
	defer closer.Close()

	parts := strings.SplitN(string(val), "\n", 2)
	if len(parts) < 1 || parts[0] == "" {
		return "", fmt.Errorf("invalid credential format")
	}
	return parts[0], nil
}

// GetEmailByUserID retrieves the email for a given userID from credentials.
// Uses the reverse index (uidx:{userID}) for O(1) lookup.
func (ts *TeamStore) GetEmailByUserID(userID string) (string, error) {
	val, closer, err := ts.db.Get(userEmailIdxKey(userID))
	if err != nil {
		return "", err
	}
	defer closer.Close()
	return string(val), nil
}

// ─── Member Self-Management ──────────────────────────────────────────────

const memberNamePrefix = "mname:"

func memberNameKey(userID string) []byte {
	return []byte(memberNamePrefix + userID)
}

// UpdateMemberPassword changes the password hash for a member credential.
// Returns an error if the email has no credential.
func (ts *TeamStore) UpdateMemberPassword(email, newPasswordHash string) error {
	val, closer, err := ts.db.Get(memberCredKey(email))
	if err != nil {
		if err == pebble.ErrNotFound {
			return fmt.Errorf("team: no credential for %q", email)
		}
		return fmt.Errorf("team: get credential: %w", err)
	}
	defer closer.Close()

	parts := strings.SplitN(string(val), "\n", 2)
	if len(parts) < 1 || parts[0] == "" {
		return fmt.Errorf("team: invalid credential format")
	}
	userID := parts[0]

	data := userID + "\n" + newPasswordHash
	return ts.db.Set(memberCredKey(email), []byte(data), pebble.Sync)
}

// GetMemberName retrieves the display name for a member.
func (ts *TeamStore) GetMemberName(userID string) (string, error) {
	val, closer, err := ts.db.Get(memberNameKey(userID))
	if err != nil {
		return "", err
	}
	defer closer.Close()
	return string(val), nil
}

// SetMemberName stores a display name for a member.
func (ts *TeamStore) SetMemberName(userID, name string) error {
	return ts.db.Set(memberNameKey(userID), []byte(name), pebble.Sync)
}

// ─── Member-Project Index ──────────────────────────────────────────────────
// Lightweight index used by GetMemberProjects to efficiently look up which
// projects a user is a member of. Indexed in the global master Pebble DB
// (same as TeamStore), updated whenever membership changes in __members.

const memberProjectIdxPrefix = "mpidx:"

func memberProjectIdxKey(userID, projectID string) []byte {
	return []byte(memberProjectIdxPrefix + userID + ":" + projectID)
}

// AddMemberProjectIndex records that a user is a member of a project.
func (ts *TeamStore) AddMemberProjectIndex(userID, projectID string) error {
	return ts.db.Set(memberProjectIdxKey(userID, projectID), []byte("1"), pebble.Sync)
}

// RemoveMemberProjectIndex removes the record that a user is a member of a project.
func (ts *TeamStore) RemoveMemberProjectIndex(userID, projectID string) error {
	return ts.db.Delete(memberProjectIdxKey(userID, projectID), pebble.Sync)
}

// GetMemberProjects returns all project IDs the given user is a member of.
// Uses the mpidx index (written when membership changes in __members).
func (ts *TeamStore) GetMemberProjects(userID string) ([]string, error) {
	prefix := []byte(memberProjectIdxPrefix + userID + ":")
	iter, err := ts.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: keyUpperBound(prefix),
	})
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var projectIDs []string
	for iter.First(); iter.Valid(); iter.Next() {
		key := string(iter.Key())
		// key format: mpidx:{userID}:{projectID}
		rest := key[len(memberProjectIdxPrefix):]
		parts := strings.SplitN(rest, ":", 2)
		if len(parts) == 2 && parts[0] == userID {
			projectIDs = append(projectIDs, parts[1])
		}
	}
	if projectIDs == nil {
		projectIDs = []string{}
	}
	return projectIDs, nil
}
