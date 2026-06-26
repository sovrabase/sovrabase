package tenant

import (
	"os"
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/ketsuna-org/sovrabase/internal/config"
)

// newTestTeamStore opens a temporary Pebble DB and wraps it in a TeamStore.
func newTestTeamStore(t *testing.T) *TeamStore {
	t.Helper()
	dir, err := os.MkdirTemp("", "sovrabase-team-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	db, err := pebble.Open(dir, &pebble.Options{})
	if err != nil {
		t.Fatalf("open pebble: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return NewTeamStore(db)
}

func TestAddMemberAndListMembers(t *testing.T) {
	ts := newTestTeamStore(t)
	pid := "proj-1"

	// Add two members
	err := ts.AddMember(pid, "user-a", RoleOwner)
	if err != nil {
		t.Fatalf("AddMember(owner): %v", err)
	}
	err = ts.AddMember(pid, "user-b", RoleDeveloper)
	if err != nil {
		t.Fatalf("AddMember(developer): %v", err)
	}

	// List them
	members, err := ts.ListMembers(pid)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}

	// Check the owner
	foundOwner := false
	foundDev := false
	for _, m := range members {
		if m.UserID == "user-a" {
			foundOwner = true
			if m.Role != RoleOwner {
				t.Errorf("user-a role = %s, want owner", m.Role)
			}
			if !m.IsOwner {
				t.Errorf("user-a IsOwner should be true")
			}
		}
		if m.UserID == "user-b" {
			foundDev = true
			if m.Role != RoleDeveloper {
				t.Errorf("user-b role = %s, want developer", m.Role)
			}
			if m.IsOwner {
				t.Errorf("user-b IsOwner should be false")
			}
		}
	}
	if !foundOwner {
		t.Error("user-a not found in members")
	}
	if !foundDev {
		t.Error("user-b not found in members")
	}
}

func TestRemoveMember(t *testing.T) {
	ts := newTestTeamStore(t)
	pid := "proj-1"

	ts.AddMember(pid, "user-a", RoleAdmin)
	ts.AddMember(pid, "user-b", RoleViewer)

	// Remove user-a
	err := ts.RemoveMember(pid, "user-a")
	if err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}

	members, _ := ts.ListMembers(pid)
	if len(members) != 1 {
		t.Fatalf("expected 1 member after removal, got %d", len(members))
	}
	if members[0].UserID != "user-b" {
		t.Errorf("remaining member should be user-b, got %s", members[0].UserID)
	}

	// Removing non-existent member should error
	err = ts.RemoveMember(pid, "nonexistent")
	if err == nil {
		t.Fatal("expected error when removing non-existent member")
	}
}

func TestUpdateMemberRole(t *testing.T) {
	ts := newTestTeamStore(t)
	pid := "proj-1"

	ts.AddMember(pid, "user-a", RoleDeveloper)

	// Promote to admin
	err := ts.UpdateMemberRole(pid, "user-a", RoleAdmin)
	if err != nil {
		t.Fatalf("UpdateMemberRole: %v", err)
	}

	member, err := ts.GetMember(pid, "user-a")
	if err != nil {
		t.Fatalf("GetMember: %v", err)
	}
	if member.Role != RoleAdmin {
		t.Errorf("role = %s, want admin", member.Role)
	}
	if member.IsOwner {
		t.Errorf("IsOwner should be false for admin")
	}

	// Promote to owner
	err = ts.UpdateMemberRole(pid, "user-a", RoleOwner)
	if err != nil {
		t.Fatalf("UpdateMemberRole(owner): %v", err)
	}
	member, _ = ts.GetMember(pid, "user-a")
	if !member.IsOwner {
		t.Errorf("IsOwner should be true for owner")
	}

	// Update non-existent member
	err = ts.UpdateMemberRole(pid, "nobody", RoleViewer)
	if err == nil {
		t.Fatal("expected error when updating non-existent member")
	}
}

func TestGetMember(t *testing.T) {
	ts := newTestTeamStore(t)
	pid := "proj-1"

	ts.AddMember(pid, "user-a", RoleViewer)

	member, err := ts.GetMember(pid, "user-a")
	if err != nil {
		t.Fatalf("GetMember: %v", err)
	}
	if member.UserID != "user-a" || member.Role != RoleViewer {
		t.Errorf("unexpected member: %+v", member)
	}

	// Non-existent
	_, err = ts.GetMember(pid, "nobody")
	if err == nil {
		t.Fatal("expected error for non-existent member")
	}
}

func TestDuplicatePrevention(t *testing.T) {
	ts := newTestTeamStore(t)
	pid := "proj-1"

	err := ts.AddMember(pid, "user-a", RoleDeveloper)
	if err != nil {
		t.Fatalf("first AddMember: %v", err)
	}

	// Duplicate should fail
	err = ts.AddMember(pid, "user-a", RoleAdmin)
	if err == nil {
		t.Fatal("expected error when adding duplicate member")
	}
}

func TestCreateInvitationAndAccept(t *testing.T) {
	ts := newTestTeamStore(t)
	pid := "proj-1"

	// Create an invitation
	inv, err := ts.CreateInvitation(pid, "test@example.com", "creator-id", RoleDeveloper, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("CreateInvitation: %v", err)
	}

	if inv.Token == "" {
		t.Fatal("expected non-empty token")
	}
	if inv.Status != InvitationPending {
		t.Fatalf("status = %s, want pending", inv.Status)
	}
	if inv.Role != RoleDeveloper {
		t.Fatalf("role = %s, want developer", inv.Role)
	}
	if inv.ProjectID != pid {
		t.Fatalf("project_id = %s, want %s", inv.ProjectID, pid)
	}
	if inv.Email != "test@example.com" {
		t.Fatalf("email = %s, want test@example.com", inv.Email)
	}
	if inv.AcceptedAt != nil {
		t.Fatal("AcceptedAt should be nil for pending invitation")
	}

	// Accept the invitation
	member, err := ts.AcceptInvitation(inv.Token, "user-a")
	if err != nil {
		t.Fatalf("AcceptInvitation: %v", err)
	}
	if member == nil {
		t.Fatal("expected non-nil member")
	}
	if member.UserID != "user-a" {
		t.Errorf("member user_id = %s, want user-a", member.UserID)
	}
	if member.Role != RoleDeveloper {
		t.Errorf("member role = %s, want developer", member.Role)
	}
	if member.ProjectID != pid {
		t.Errorf("member project_id = %s, want %s", member.ProjectID, pid)
	}

	// Verify the invitation is now accepted
	inv2, err := ts.GetInvitation(inv.Token)
	if err != nil {
		t.Fatalf("GetInvitation after accept: %v", err)
	}
	if inv2.Status != InvitationAccepted {
		t.Fatalf("invitation status = %s, want accepted", inv2.Status)
	}
	if inv2.AcceptedAt == nil {
		t.Fatal("AcceptedAt should not be nil after acceptance")
	}

	// Verify the user is now a member
	if !ts.IsMember(pid, "user-a") {
		t.Fatal("user-a should be a member after accepting invitation")
	}
}

func TestGetInvitationPending(t *testing.T) {
	ts := newTestTeamStore(t)
	pid := "proj-1"

	inv, err := ts.CreateInvitation(pid, "p@test.com", "creator", RoleViewer, time.Hour)
	if err != nil {
		t.Fatalf("CreateInvitation: %v", err)
	}

	got, err := ts.GetInvitation(inv.Token)
	if err != nil {
		t.Fatalf("GetInvitation: %v", err)
	}
	if got.Status != InvitationPending {
		t.Fatalf("status = %s, want pending", got.Status)
	}
	if got.AcceptedAt != nil {
		t.Fatal("AcceptedAt should be nil for pending invitation")
	}
	if got.Token != inv.Token {
		t.Fatal("token mismatch")
	}
}

func TestGetInvitationAccepted(t *testing.T) {
	ts := newTestTeamStore(t)
	pid := "proj-1"

	inv, _ := ts.CreateInvitation(pid, "a@b.com", "creator", RoleAdmin, time.Hour)
	ts.AcceptInvitation(inv.Token, "user-x")

	got, err := ts.GetInvitation(inv.Token)
	if err != nil {
		t.Fatalf("GetInvitation: %v", err)
	}
	if got.Status != InvitationAccepted {
		t.Fatalf("status = %s, want accepted", got.Status)
	}
	if got.AcceptedAt == nil {
		t.Fatal("AcceptedAt should be set for accepted invitation")
	}
}

func TestRevokeInvitation(t *testing.T) {
	ts := newTestTeamStore(t)
	pid := "proj-1"

	inv, err := ts.CreateInvitation(pid, "revoke@test.com", "creator", RoleViewer, time.Hour)
	if err != nil {
		t.Fatalf("CreateInvitation: %v", err)
	}

	// Revoke
	err = ts.RevokeInvitation(inv.Token)
	if err != nil {
		t.Fatalf("RevokeInvitation: %v", err)
	}

	got, err := ts.GetInvitation(inv.Token)
	if err != nil {
		t.Fatalf("GetInvitation after revoke: %v", err)
	}
	if got.Status != InvitationRevoked {
		t.Fatalf("status = %s, want revoked", got.Status)
	}

	// Revoking an already revoked invitation should fail
	err = ts.RevokeInvitation(inv.Token)
	if err == nil {
		t.Fatal("expected error when revoking already revoked invitation")
	}

	// Revoking a non-existent invitation should fail
	err = ts.RevokeInvitation("nonexistent-token")
	if err == nil {
		t.Fatal("expected error when revoking non-existent invitation")
	}
}

func TestAcceptExpiredInvitation(t *testing.T) {
	ts := newTestTeamStore(t)

	// Use a zero-length TTL so the invitation is already expired
	inv, err := ts.CreateInvitation("proj-x", "expired@test.com", "creator", RoleDeveloper, -1*time.Hour)
	if err != nil {
		t.Fatalf("CreateInvitation: %v", err)
	}

	_, err = ts.AcceptInvitation(inv.Token, "user-z")
	if err == nil {
		t.Fatal("expected error when accepting expired invitation")
	}
}

func TestAcceptAlreadyAcceptedInvitation(t *testing.T) {
	ts := newTestTeamStore(t)
	pid := "proj-y"

	inv, _ := ts.CreateInvitation(pid, "double@test.com", "creator", RoleAdmin, time.Hour)

	// First accept succeeds
	_, err := ts.AcceptInvitation(inv.Token, "user-1")
	if err != nil {
		t.Fatalf("first AcceptInvitation: %v", err)
	}

	// Second accept should fail
	_, err = ts.AcceptInvitation(inv.Token, "user-2")
	if err == nil {
		t.Fatal("expected error when accepting already accepted invitation")
	}
}

func TestIsMember(t *testing.T) {
	ts := newTestTeamStore(t)
	pid := "proj-1"

	ts.AddMember(pid, "user-a", RoleDeveloper)

	if !ts.IsMember(pid, "user-a") {
		t.Error("user-a should be a member")
	}
	if ts.IsMember(pid, "nobody") {
		t.Error("nobody should not be a member")
	}
	if ts.IsMember("other-proj", "user-a") {
		t.Error("user-a should not be a member of other-proj")
	}
}

func TestHasRole(t *testing.T) {
	ts := newTestTeamStore(t)
	pid := "proj-1"

	ts.AddMember(pid, "admin-user", RoleAdmin)
	ts.AddMember(pid, "dev-user", RoleDeveloper)
	ts.AddMember(pid, "view-user", RoleViewer)

	// Single role check
	if !ts.HasRole(pid, "admin-user", RoleAdmin) {
		t.Error("admin-user should have RoleAdmin")
	}
	if ts.HasRole(pid, "admin-user", RoleDeveloper) {
		t.Error("admin-user should not have RoleDeveloper")
	}

	// Multiple allowed roles
	if !ts.HasRole(pid, "dev-user", RoleAdmin, RoleDeveloper, RoleViewer) {
		t.Error("dev-user should match developer role")
	}
	if ts.HasRole(pid, "dev-user", RoleOwner, RoleAdmin) {
		t.Error("dev-user should not match owner or admin")
	}

	// Non-member
	if ts.HasRole(pid, "nobody", RoleViewer) {
		t.Error("non-member should not have any role")
	}
}

func TestListInvitations(t *testing.T) {
	ts := newTestTeamStore(t)
	pid := "proj-1"

	// Create a few invitations
	inv1, _ := ts.CreateInvitation(pid, "a@test.com", "creator", RoleDeveloper, time.Hour)
	inv2, _ := ts.CreateInvitation(pid, "b@test.com", "creator", RoleAdmin, time.Hour)
	inv3, _ := ts.CreateInvitation("other-proj", "c@test.com", "creator", RoleViewer, time.Hour)

	// List for proj-1
	invs, err := ts.ListInvitations(pid)
	if err != nil {
		t.Fatalf("ListInvitations: %v", err)
	}
	if len(invs) != 2 {
		t.Fatalf("expected 2 invitations for proj-1, got %d", len(invs))
	}

	// Verify both are present
	tokens := map[string]bool{inv1.Token: true, inv2.Token: true}
	for _, inv := range invs {
		delete(tokens, inv.Token)
		if inv.ProjectID != pid {
			t.Errorf("invitation belongs to project %s, want %s", inv.ProjectID, pid)
		}
	}
	if len(tokens) > 0 {
		t.Errorf("expected invitations not found: %v", tokens)
	}

	// List for other-proj
	otherInvs, _ := ts.ListInvitations("other-proj")
	if len(otherInvs) != 1 {
		t.Fatalf("expected 1 invitation for other-proj, got %d", len(otherInvs))
	}
	if otherInvs[0].Token != inv3.Token {
		t.Error("wrong invitation token for other-proj")
	}

	// Accept one invitation and verify it shows in list
	ts.AcceptInvitation(inv1.Token, "user-accepted")
	acceptedInvs, _ := ts.ListInvitations(pid)
	if len(acceptedInvs) != 2 {
		t.Fatalf("expected 2 invitations after accept (still listed), got %d", len(acceptedInvs))
	}
}

func TestOwnerAutoAddedOnCreateProject(t *testing.T) {
	// Use ProjectManager to verify that CreateProject auto-adds the owner
	dir, err := os.MkdirTemp("", "sovrabase-tenant-owner-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	pm, err := NewProjectManager(dir, &config.Config{JWTSecret: "this-is-a-very-long-test-secret-key-for-testing"})
	if err != nil {
		t.Fatalf("NewProjectManager: %v", err)
	}
	t.Cleanup(func() { pm.Close() })

	proj, err := pm.CreateProject("owner-test", "owner-user-id")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	ts := pm.GetTeamStore()
	if !ts.IsMember(proj.ID, "owner-user-id") {
		t.Fatal("owner should be a team member after CreateProject")
	}

	member, err := ts.GetMember(proj.ID, "owner-user-id")
	if err != nil {
		t.Fatalf("GetMember: %v", err)
	}
	if member.Role != RoleOwner {
		t.Errorf("owner role = %s, want owner", member.Role)
	}
	if !member.IsOwner {
		t.Error("owner IsOwner should be true")
	}
}
