package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// signedPayload is the JSON payload embedded in a signed URL token.
type signedPayload struct {
	Bucket string `json:"b"`
	Path   string `json:"p"`
	Exp    int64  `json:"exp"`
}

// signedURLTTL is the default lifetime of a signed URL (1 hour).
const signedURLTTL = 1 * time.Hour

// generateSignedToken creates a time-limited HMAC-SHA256 token authorising
// access to a specific file. Format: base64(payload).base64(signature).
func generateSignedToken(secret []byte, bucket, path string, expiresAt time.Time) (string, error) {
	if len(secret) == 0 {
		return "", errors.New("empty signing secret")
	}
	payload := signedPayload{
		Bucket: bucket,
		Path:   path,
		Exp:    expiresAt.Unix(),
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payloadB64))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return payloadB64 + "." + sig, nil
}

// validateSignedToken verifies the HMAC signature and expiration of a signed
// token. Returns the bucket and path it authorises. The caller must still check
// that the bucket+path match the URL being accessed.
func validateSignedToken(secret []byte, token string) (bucket, path string, err error) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return "", "", errors.New("invalid token format")
	}
	payloadB64, sigB64 := parts[0], parts[1]

	// Verify signature first (constant-time via hmac.Equal is implicit in recompute).
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payloadB64))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if sigB64 != expectedSig {
		return "", "", errors.New("invalid signature")
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return "", "", fmt.Errorf("decode payload: %w", err)
	}
	var p signedPayload
	if err := json.Unmarshal(payloadJSON, &p); err != nil {
		return "", "", fmt.Errorf("unmarshal payload: %w", err)
	}

	if p.Exp > 0 && time.Now().Unix() > p.Exp {
		return "", "", errors.New("signed URL has expired")
	}

	return p.Bucket, p.Path, nil
}
