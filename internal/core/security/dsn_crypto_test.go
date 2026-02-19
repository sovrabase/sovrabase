package security

import (
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestDSNCipherRoundTrip(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	cipher, err := NewDSNCipher(key)
	if err != nil {
		t.Fatalf("NewDSNCipher() error = %v", err)
	}

	encrypted, err := cipher.Encrypt("postgres://user:pass@localhost:5432/db")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	decrypted, err := cipher.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if decrypted != "postgres://user:pass@localhost:5432/db" {
		t.Fatalf("Decrypt() = %q, want original value", decrypted)
	}
}

func TestDecodeMasterKeyFormats(t *testing.T) {
	raw := "0123456789abcdef0123456789abcdef"
	hexValue := hex.EncodeToString([]byte(raw))
	base64Value := base64.StdEncoding.EncodeToString([]byte(raw))

	tests := []struct {
		name  string
		input string
	}{
		{name: "raw", input: raw},
		{name: "hex", input: hexValue},
		{name: "base64", input: base64Value},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := DecodeMasterKey(tt.input)
			if err != nil {
				t.Fatalf("DecodeMasterKey() error = %v", err)
			}
			if len(key) != 32 {
				t.Fatalf("DecodeMasterKey() length = %d, want 32", len(key))
			}
		})
	}
}

func TestDecodeMasterKeyRejectsInvalidValue(t *testing.T) {
	if _, err := DecodeMasterKey("invalid"); err == nil {
		t.Fatalf("DecodeMasterKey() error = nil, want non-nil")
	}
}

func TestDSNCipherRejectsCorruptedCiphertext(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	cipher, err := NewDSNCipher(key)
	if err != nil {
		t.Fatalf("NewDSNCipher() error = %v", err)
	}

	encrypted, err := cipher.Encrypt("mongodb://user:pass@localhost:27017/db")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	encrypted = encrypted + "broken"
	if _, err := cipher.Decrypt(encrypted); err == nil {
		t.Fatalf("Decrypt() error = nil, want non-nil for corrupted payload")
	}
}
