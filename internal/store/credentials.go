package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	credentialKeyBytes = 32
	passwordVersionV1  = "v1:"
)

func loadOrCreateCredentialKey(dbPath string) ([]byte, error) {
	keyPath := dbPath + ".key"
	if raw, err := os.ReadFile(keyPath); err == nil {
		key, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(raw)))
		if err != nil {
			return nil, fmt.Errorf("decode %s: %w", keyPath, err)
		}
		if len(key) != credentialKeyBytes {
			return nil, fmt.Errorf("decode %s: got %d bytes, want %d", keyPath, len(key), credentialKeyBytes)
		}
		return key, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read %s: %w", keyPath, err)
	}

	key := make([]byte, credentialKeyBytes)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o755); err != nil {
		return nil, err
	}
	encoded := base64.RawStdEncoding.EncodeToString(key)
	if err := os.WriteFile(keyPath, []byte(encoded+"\n"), 0o600); err != nil {
		return nil, fmt.Errorf("write %s: %w", keyPath, err)
	}
	return key, nil
}

func (d *DB) encodePassword(plain string) (string, error) {
	aead, err := d.credentialAEAD()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ciphertext := aead.Seal(nonce, nonce, []byte(plain), nil)
	return passwordVersionV1 + base64.RawStdEncoding.EncodeToString(ciphertext), nil
}

func (d *DB) decodePassword(stored string) (string, error) {
	if !strings.HasPrefix(stored, passwordVersionV1) {
		return "", fmt.Errorf("unsupported stored password format")
	}
	aead, err := d.credentialAEAD()
	if err != nil {
		return "", err
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(stored, passwordVersionV1))
	if err != nil {
		return "", fmt.Errorf("decode stored password: %w", err)
	}
	if len(raw) < aead.NonceSize() {
		return "", fmt.Errorf("decode stored password: ciphertext too short")
	}
	nonce, ciphertext := raw[:aead.NonceSize()], raw[aead.NonceSize():]
	plain, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt stored password: %w", err)
	}
	return string(plain), nil
}

func (d *DB) credentialAEAD() (cipher.AEAD, error) {
	if len(d.credentialKey) != credentialKeyBytes {
		return nil, fmt.Errorf("credential key has %d bytes, want %d", len(d.credentialKey), credentialKeyBytes)
	}
	block, err := aes.NewCipher(d.credentialKey)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
