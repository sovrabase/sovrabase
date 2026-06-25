package auth

import (
	"testing"
	"time"
)

func TestGenerateMFASecret(t *testing.T) {
	secret, err := GenerateMFASecret()
	if err != nil {
		t.Fatalf("GenerateMFASecret failed: %v", err)
	}
	if len(secret) < 16 {
		t.Errorf("secret too short: %d chars", len(secret))
	}

	// Two secrets should be different.
	secret2, _ := GenerateMFASecret()
	if secret == secret2 {
		t.Error("two secrets should be different")
	}
}

func TestTOTPKnownVector(t *testing.T) {
	// RFC 6238 test vectors use a known secret and check specific codes.
	// Secret: ASCII "12345678901234567890" → base32: GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

	// Known T=59 → counter=1 → expected code: 287082
	code := computeTOTP(secret, 1)
	if code != "287082" {
		t.Errorf("T=59: expected 287082, got %s", code)
	}

	// Known T=1111111109 → counter=37037036 → expected code: 081804
	code = computeTOTP(secret, 37037036)
	if code != "081804" {
		t.Errorf("T=1111111109: expected 081804, got %s", code)
	}
}

func TestValidateTOTP(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP" // test secret

	// Compute the code for the current time window.
	now := time.Now().Unix()
	counter := now / 30
	code := computeTOTP(secret, counter)

	// Should validate within the current window (±1 period).
	if !ValidateTOTP(secret, code) {
		t.Errorf("expected code %s to validate for current time", code)
	}
}

func TestValidateTOTPInvalidCode(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"

	if ValidateTOTP(secret, "000000") {
		// 000000 could theoretically match, but very unlikely.
		// Run a few more to reduce false positive chance.
		for i := 0; i < 5; i++ {
			if !ValidateTOTP(secret, "999999") {
				return // at least one should fail
			}
		}
		t.Error("expected random codes to mostly fail validation")
	}
}

func TestGenerateOTPUri(t *testing.T) {
	uri := GenerateOTPUri("JBSWY3DPEHPK3PXP", "user@example.com", "TestApp")
	if !contains(uri, "otpauth://totp/") {
		t.Errorf("URI doesn't start with otpauth://totp/: %s", uri)
	}
	if !contains(uri, "secret=JBSWY3DPEHPK3PXP") {
		t.Errorf("URI doesn't contain secret: %s", uri)
	}
	if !contains(uri, "issuer=TestApp") {
		t.Errorf("URI doesn't contain issuer: %s", uri)
	}
}

func TestGenerateBackupCodes(t *testing.T) {
	codes, err := GenerateBackupCodes()
	if err != nil {
		t.Fatalf("GenerateBackupCodes failed: %v", err)
	}
	if len(codes) != 8 {
		t.Errorf("expected 8 codes, got %d", len(codes))
	}

	// Check format: XXXX-XXXX (9 chars).
	for _, code := range codes {
		if len(code) != 9 {
			t.Errorf("code %q has wrong length: %d", code, len(code))
		}
		if code[4] != '-' {
			t.Errorf("code %q missing dash at position 4", code)
		}
	}

	// Codes should be unique.
	seen := make(map[string]bool)
	for _, code := range codes {
		if seen[code] {
			t.Errorf("duplicate backup code: %s", code)
		}
		seen[code] = true
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
