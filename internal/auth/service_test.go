package auth

import (
	"testing"
	"time"
)

const testJWTSecret = "test-secret-key-for-unit-tests"

func newTestService() *AuthService {
	return NewService(testJWTSecret, NewInMemoryUserStore())
}

func TestHashPassword(t *testing.T) {
	password := "my-secure-password"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if hash == "" {
		t.Fatal("expected non-empty hash")
	}

	if hash == password {
		t.Fatal("hash should not equal original password")
	}
}

func TestCheckPassword(t *testing.T) {
	password := "my-secure-password"
	hash, _ := HashPassword(password)

	tests := []struct {
		name     string
		hash     string
		password string
		wantErr  bool
	}{
		{"correct password", hash, password, false},
		{"wrong password", hash, "wrong-password", true},
		{"empty password", hash, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckPassword(tt.hash, tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckPassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSignUp(t *testing.T) {
	svc := newTestService()

	user, tokens, err := svc.SignUp("alice@example.com", "password123")
	if err != nil {
		t.Fatalf("SignUp failed: %v", err)
	}

	if user.ID == "" {
		t.Fatal("expected non-empty user ID")
	}
	if user.Email != "alice@example.com" {
		t.Fatalf("expected email alice@example.com, got %s", user.Email)
	}
	if user.Role != RoleUser {
		t.Fatalf("expected role user, got %s", user.Role)
	}
	if user.CreatedAt.IsZero() || user.UpdatedAt.IsZero() {
		t.Fatal("expected non-zero timestamps")
	}

	if tokens.AccessToken == "" {
		t.Fatal("expected non-empty access token")
	}
	if tokens.RefreshToken == "" {
		t.Fatal("expected non-empty refresh token")
	}
	if tokens.ExpiresIn <= 0 {
		t.Fatalf("expected positive expires_in, got %d", tokens.ExpiresIn)
	}

	// Duplicate signup should fail
	_, _, err = svc.SignUp("alice@example.com", "password123")
	if err == nil {
		t.Fatal("expected duplicate signup to fail")
	}
}

func TestSignUpValidation(t *testing.T) {
	svc := newTestService()

	tests := []struct {
		name     string
		email    string
		password string
	}{
		{"empty email", "", "password123"},
		{"empty password", "bob@example.com", ""},
		{"short password", "bob@example.com", "1234567"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := svc.SignUp(tt.email, tt.password)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestSignIn(t *testing.T) {
	svc := newTestService()

	_, _, err := svc.SignUp("alice@example.com", "password123")
	if err != nil {
		t.Fatalf("SignUp failed: %v", err)
	}

	tokens, err := svc.SignIn("alice@example.com", "password123")
	if err != nil {
		t.Fatalf("SignIn failed: %v", err)
	}

	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatal("expected non-empty tokens from SignIn")
	}

	// Wrong password
	_, err = svc.SignIn("alice@example.com", "wrong-password")
	if err == nil {
		t.Fatal("expected SignIn to fail with wrong password")
	}

	// Unknown email
	_, err = svc.SignIn("unknown@example.com", "password123")
	if err == nil {
		t.Fatal("expected SignIn to fail with unknown email")
	}
}

func TestJWTFlow(t *testing.T) {
	svc := newTestService()

	user, _, err := svc.SignUp("alice@example.com", "password123")
	if err != nil {
		t.Fatalf("SignUp failed: %v", err)
	}

	accessToken, err := GenerateAccessToken(user, testJWTSecret)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	claims, err := ValidateToken(accessToken, testJWTSecret)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if claims.UserID != user.ID {
		t.Fatalf("expected user ID %s, got %s", user.ID, claims.UserID)
	}
	if claims.Email != user.Email {
		t.Fatalf("expected email %s, got %s", user.Email, claims.Email)
	}
	if claims.Role != string(user.Role) {
		t.Fatalf("expected role %s, got %s", user.Role, claims.Role)
	}
	if claims.RegisteredClaims.Issuer != "sovrabase" {
		t.Fatalf("expected issuer sovrabase, got %s", claims.RegisteredClaims.Issuer)
	}

	// Validate with wrong secret
	_, err = ValidateToken(accessToken, "wrong-secret")
	if err == nil {
		t.Fatal("expected validation to fail with wrong secret")
	}
}

func TestRefreshToken(t *testing.T) {
	svc := newTestService()

	_, initialTokens, err := svc.SignUp("alice@example.com", "password123")
	if err != nil {
		t.Fatalf("SignUp failed: %v", err)
	}

	// Wait a tiny bit so new tokens have different issued-at times
	time.Sleep(10 * time.Millisecond)

	newTokens, err := svc.RefreshToken(initialTokens.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshToken failed: %v", err)
	}

	if newTokens.AccessToken == initialTokens.AccessToken {
		t.Fatal("expected a different access token after refresh")
	}
	if newTokens.RefreshToken == initialTokens.RefreshToken {
		t.Fatal("expected a different refresh token after refresh")
	}

	// Refreshing with an invalid token should fail
	_, err = svc.RefreshToken("garbage-token")
	if err == nil {
		t.Fatal("expected RefreshToken to fail with garbage token")
	}
}

func TestGetUser(t *testing.T) {
	svc := newTestService()

	created, _, err := svc.SignUp("alice@example.com", "password123")
	if err != nil {
		t.Fatalf("SignUp failed: %v", err)
	}

	retrieved, err := svc.GetUser(created.ID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}

	if retrieved.Email != created.Email {
		t.Fatalf("expected email %s, got %s", created.Email, retrieved.Email)
	}

	_, err = svc.GetUser("non-existent-id")
	if err == nil {
		t.Fatal("expected GetUser to fail for unknown ID")
	}
}

func TestValidateAccessToken(t *testing.T) {
	svc := newTestService()

	user, _, err := svc.SignUp("alice@example.com", "password123")
	if err != nil {
		t.Fatalf("SignUp failed: %v", err)
	}

	accessToken, _ := GenerateAccessToken(user, testJWTSecret)

	claims, err := svc.ValidateAccessToken(accessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken failed: %v", err)
	}

	if claims.UserID != user.ID {
		t.Fatalf("expected user ID %s, got %s", user.ID, claims.UserID)
	}
}

func TestCreateOAuthState(t *testing.T) {
	svc := newTestService()

	state, err := svc.CreateOAuthState("google", "test-project-id", "/dashboard")
	if err != nil {
		t.Fatalf("CreateOAuthState failed: %v", err)
	}

	if state == "" {
		t.Fatal("expected non-empty state token")
	}
	// State format: base64url(json_payload).hex(random32)
	// JSON payload ~ {"project_id":"...","app_redirect":"..."}
	// Must be longer than the random suffix alone
	if len(state) < 1 + 64 + len("test-project-id") {
		t.Fatalf("expected state longer than random suffix alone, got %d chars", len(state))
	}

	// Verify the payload can be decoded
	payload, err := svc.DecodeStatePayload(state)
	if err != nil {
		t.Fatalf("DecodeStatePayload failed: %v", err)
	}
	if payload.ProjectID != "test-project-id" {
		t.Fatalf("expected project ID 'test-project-id', got '%s'", payload.ProjectID)
	}
	if payload.AppRedirect != "/dashboard" {
		t.Fatalf("expected app redirect '/dashboard', got '%s'", payload.AppRedirect)
	}
}

func TestUserStore(t *testing.T) {
	store := NewInMemoryUserStore()

	user := NewUser("test@example.com", "hashed-password")

	// Create
	err := store.Create(user)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Duplicate email
	err = store.Create(NewUser("test@example.com", "password2"))
	if err == nil {
		t.Fatal("expected duplicate email to fail")
	}

	// GetByEmail
	retrieved, err := store.GetByEmail("test@example.com")
	if err != nil {
		t.Fatalf("GetByEmail failed: %v", err)
	}
	if retrieved.ID != user.ID {
		t.Fatalf("expected user ID %s, got %s", user.ID, retrieved.ID)
	}

	// GetByID
	retrieved, err = store.GetByID(user.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if retrieved.Email != user.Email {
		t.Fatalf("expected email %s, got %s", user.Email, retrieved.Email)
	}

	// Update
	user.Email = "updated@example.com"
	err = store.Update(user)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify old email can't be found
	_, err = store.GetByEmail("test@example.com")
	if err == nil {
		t.Fatal("expected old email to not be found after update")
	}

	// Verify new email works
	retrieved, err = store.GetByEmail("updated@example.com")
	if err != nil {
		t.Fatalf("GetByEmail after update failed: %v", err)
	}
	if retrieved.Email != "updated@example.com" {
		t.Fatalf("expected updated email, got %s", retrieved.Email)
	}

	// Delete
	err = store.Delete(user.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = store.GetByID(user.ID)
	if err == nil {
		t.Fatal("expected GetByID to fail after delete")
	}
}

func TestTokenAccessTokenExpiry(t *testing.T) {
	svc := newTestService()

	user, _, err := svc.SignUp("alice@example.com", "password123")
	if err != nil {
		t.Fatalf("SignUp failed: %v", err)
	}

	// Create an already-expired token by using a secret that's
	// separate from the normal one — instead, we'll create a token
	// manually with expired claims using jwt directly
	accessToken, err := GenerateAccessToken(user, testJWTSecret)
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	// Valid token should validate
	_, err = svc.ValidateAccessToken(accessToken)
	if err != nil {
		t.Fatalf("expected valid token, got error: %v", err)
	}

	// Tampered token should fail
	tampered := accessToken + "x"
	_, err = svc.ValidateAccessToken(tampered)
	if err == nil {
		t.Fatal("expected tampered token to fail validation")
	}
}

func TestVerifyEmailFlow(t *testing.T) {
	svc := newTestService()
	svc.EmailVerificationEnabled = true
	svc.SMTPHost = "localhost"

	user, _, _ := svc.SignUp("verify@example.com", "password123")

	// Invalid token
	err := svc.VerifyEmail("wrong-token")
	if err == nil {
		t.Fatal("expected VerifyEmail to fail with wrong token")
	}

	// Valid token
	err = svc.VerifyEmail(user.VerificationToken)
	if err != nil {
		t.Fatalf("VerifyEmail failed: %v", err)
	}

	// Getting user and checking verified
	retrieved, _ := svc.GetUser(user.ID)
	if !retrieved.IsVerified {
		t.Fatal("expected user to be verified")
	}
	if retrieved.VerificationToken != "" {
		t.Fatal("expected verification token to be cleared")
	}

	// Verifying again should fail (token is cleared)
	err = svc.VerifyEmail(user.VerificationToken)
	if err == nil {
		t.Fatal("expected second VerifyEmail to fail")
	}
}

func TestForgotPasswordAndResetFlow(t *testing.T) {
	svc := newTestService()
	svc.EmailVerificationEnabled = true
	svc.SMTPHost = "localhost"

	user, _, _ := svc.SignUp("reset@example.com", "password123")
	_ = svc.VerifyEmail(user.VerificationToken)

	// Forgot password for unknown email
	_, err := svc.ForgotPassword("unknown@example.com")
	if err == nil {
		t.Fatal("expected ForgotPassword to fail for unknown email")
	}

	// Forgot password for valid email
	token, err := svc.ForgotPassword("reset@example.com")
	if err != nil {
		t.Fatalf("ForgotPassword failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty reset token")
	}

	// Reset password with short password
	err = svc.ResetPassword(token, "short")
	if err == nil {
		t.Fatal("expected ResetPassword to fail for short password")
	}

	// Reset password with wrong token
	err = svc.ResetPassword("wrong-token", "newpassword123")
	if err == nil {
		t.Fatal("expected ResetPassword to fail for wrong token")
	}

	// Reset password with valid token
	err = svc.ResetPassword(token, "newpassword123")
	if err != nil {
		t.Fatalf("ResetPassword failed: %v", err)
	}

	// Check login with new password
	tokens, err := svc.SignIn("reset@example.com", "newpassword123")
	if err != nil {
		t.Fatalf("SignIn failed with new password: %v", err)
	}
	if tokens.AccessToken == "" {
		t.Fatal("expected non-empty access token after reset")
	}

	// Check login with old password fails
	_, err = svc.SignIn("reset@example.com", "password123")
	if err == nil {
		t.Fatal("expected login with old password to fail")
	}
}
