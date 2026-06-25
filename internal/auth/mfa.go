package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

// MFASecret is the base32-encoded shared secret for TOTP.
// It is stored on the user record when MFA is enabled.

// GenerateMFASecret creates a new random 20-byte TOTP secret, base32-encoded.
func GenerateMFASecret() (string, error) {
	secret := make([]byte, 20)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("generating MFA secret: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret), nil
}

// GenerateOTPUri builds the otpauth:// URI for QR code generation.
// This URI can be encoded as a QR code that authenticator apps can scan.
func GenerateOTPUri(secret, accountName, issuer string) string {
	if issuer == "" {
		issuer = "Sovrabase"
	}
	// URL-encode the label.
	label := fmt.Sprintf("%s:%s", issuer, accountName)
	return fmt.Sprintf("otpauth://totp/%s?secret=%s&issuer=%s&algorithm=SHA1&digits=6&period=30",
		label, secret, issuer)
}

// ValidateTOTP checks whether the given 6-digit code is valid for the secret
// at the current time, allowing a ±1 window skew (30 seconds each direction).
func ValidateTOTP(secret, code string) bool {
	return ValidateTOTPWithWindow(secret, code, 1)
}

// ValidateTOTPWithWindow checks the code against the given number of periods
// (each 30 seconds) before and after the current time.
func ValidateTOTPWithWindow(secret, code string, window int64) bool {
	if len(code) != 6 {
		return false
	}

	now := time.Now().Unix()
	period := int64(30)

	// Check current period and ±window periods.
	for offset := -window; offset <= window; offset++ {
		counter := (now / period) + offset
		expected := computeTOTP(secret, counter)
		if hmac.Equal([]byte(expected), []byte(code)) {
			return true
		}
	}
	return false
}

// computeTOTP generates the 6-digit TOTP code for the given counter.
func computeTOTP(secret string, counter int64) string {
	// Decode the base32 secret.
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return ""
	}

	// Convert counter to 8-byte big-endian.
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(counter))

	// HMAC-SHA1.
	mac := hmac.New(sha1.New, key)
	mac.Write(buf[:])
	hash := mac.Sum(nil)

	// Dynamic truncation.
	offset := hash[len(hash)-1] & 0x0f
	code := binary.BigEndian.Uint32(hash[offset:offset+4]) & 0x7fffffff

	// 6 digits.
	return fmt.Sprintf("%06d", code%1000000)
}

// GenerateBackupCodes creates 8 single-use backup codes for MFA recovery.
// Each code is 8 characters (4 groups of 4 hex chars separated by dashes).
func GenerateBackupCodes() ([]string, error) {
	var codes []string
	for i := 0; i < 8; i++ {
		b := make([]byte, 4)
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("generating backup code: %w", err)
		}
		code := fmt.Sprintf("%x", b)
		// Format as XXXX-XXXX.
		code = fmt.Sprintf("%s-%s", code[:4], code[4:])
		codes = append(codes, code)
	}
	return codes, nil
}
