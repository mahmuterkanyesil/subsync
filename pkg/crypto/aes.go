package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
)

// EncryptValue encrypts plaintext using AES-256-GCM with a key derived from secret.
// Returns a base64-encoded ciphertext (nonce prepended).
func EncryptValue(plaintext, secret string) (string, error) {
	if secret == "" {
		return plaintext, nil
	}
	block, err := newCipher(secret)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptValue decrypts a value previously produced by EncryptValue.
// If secret is empty the value is returned as-is (unencrypted passthrough).
func DecryptValue(encoded, secret string) (string, error) {
	if secret == "" {
		return encoded, nil
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		// Value was likely stored before encryption was enabled — return as-is.
		return encoded, nil
	}
	block, err := newCipher(secret)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		// Decryption failed — value may be plaintext from before encryption was enabled.
		return encoded, nil
	}
	return string(plaintext), nil
}

func newCipher(secret string) (cipher.Block, error) {
	key := sha256.Sum256([]byte(secret))
	return aes.NewCipher(key[:])
}
