package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	cipherPrefix = "enc:v1:"
	masterKeyLen = 32
)

type DSNCipher struct {
	aead cipher.AEAD
}

func NewDSNCipher(masterKey []byte) (*DSNCipher, error) {
	if len(masterKey) != masterKeyLen {
		return nil, fmt.Errorf("master key must be %d bytes", masterKeyLen)
	}

	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("create AES cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM cipher: %w", err)
	}

	return &DSNCipher{aead: aead}, nil
}

func ResolveMasterKeyFromEnv(name string) ([]byte, error) {
	value := os.Getenv(name)
	if strings.TrimSpace(value) == "" {
		return nil, fmt.Errorf("missing required master key env %q", name)
	}
	return DecodeMasterKey(value)
}

func DecodeMasterKey(raw string) ([]byte, error) {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) == masterKeyLen {
		return []byte(trimmed), nil
	}

	if len(trimmed) == masterKeyLen*2 {
		buf, err := hex.DecodeString(trimmed)
		if err == nil && len(buf) == masterKeyLen {
			return buf, nil
		}
	}

	buf, err := base64.StdEncoding.DecodeString(trimmed)
	if err == nil && len(buf) == masterKeyLen {
		return buf, nil
	}

	buf, err = base64.RawStdEncoding.DecodeString(trimmed)
	if err == nil && len(buf) == masterKeyLen {
		return buf, nil
	}

	return nil, errors.New("master key must be 32-byte raw, 64-char hex, or base64-encoded 32 bytes")
}

func (c *DSNCipher) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", errors.New("plaintext cannot be empty")
	}

	nonce := make([]byte, c.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := c.aead.Seal(nil, nonce, []byte(plaintext), nil)
	packed := append(nonce, ciphertext...)
	return cipherPrefix + base64.RawStdEncoding.EncodeToString(packed), nil
}

func (c *DSNCipher) Decrypt(encoded string) (string, error) {
	if !strings.HasPrefix(encoded, cipherPrefix) {
		return "", errors.New("ciphertext missing enc:v1 prefix")
	}

	raw := strings.TrimPrefix(encoded, cipherPrefix)
	packed, err := base64.RawStdEncoding.DecodeString(raw)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	nonceSize := c.aead.NonceSize()
	if len(packed) <= nonceSize {
		return "", errors.New("ciphertext payload is too short")
	}

	nonce := packed[:nonceSize]
	ciphertext := packed[nonceSize:]

	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt ciphertext: %w", err)
	}
	return string(plaintext), nil
}
