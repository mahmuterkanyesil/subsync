package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	secret := "super-secret-key"
	plaintext := "AIzaSyD-example-api-key"

	enc, err := EncryptValue(plaintext, secret)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, enc)

	dec, err := DecryptValue(enc, secret)
	require.NoError(t, err)
	assert.Equal(t, plaintext, dec)
}

func TestEncryptDecrypt_EmptySecret_Passthrough(t *testing.T) {
	plaintext := "my-api-key"
	enc, err := EncryptValue(plaintext, "")
	require.NoError(t, err)
	assert.Equal(t, plaintext, enc)

	dec, err := DecryptValue(enc, "")
	require.NoError(t, err)
	assert.Equal(t, plaintext, dec)
}

func TestEncryptDecrypt_ProducesUniqueCiphertexts(t *testing.T) {
	secret := "key"
	plaintext := "value"

	enc1, err := EncryptValue(plaintext, secret)
	require.NoError(t, err)
	enc2, err := EncryptValue(plaintext, secret)
	require.NoError(t, err)

	// Random nonce ensures different ciphertexts each time.
	assert.NotEqual(t, enc1, enc2)
}

func TestDecryptValue_PlaintextFallback(t *testing.T) {
	// A pre-existing plaintext value (not base64 GCM) should be returned as-is
	// when secret is set, enabling a zero-downtime migration.
	dec, err := DecryptValue("plain-old-key", "some-secret")
	require.NoError(t, err)
	assert.Equal(t, "plain-old-key", dec)
}

func TestDecryptValue_WrongSecret_Fallback(t *testing.T) {
	enc, err := EncryptValue("secret-value", "correct-secret")
	require.NoError(t, err)

	// Wrong secret fails GCM auth tag — should return the encoded value as-is, not an error.
	dec, err := DecryptValue(enc, "wrong-secret")
	require.NoError(t, err)
	assert.Equal(t, enc, dec)
}
