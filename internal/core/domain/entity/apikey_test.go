package entity_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"subsync/internal/core/domain/entity"
	"subsync/internal/core/domain/exception"
)

func TestNewAPIKey_Valid(t *testing.T) {
	k, err := entity.NewAPIKey("gemini", "key-abc-123")

	require.NoError(t, err)
	assert.Equal(t, "gemini", k.Service())
	assert.Equal(t, "key-abc-123", k.KeyValue())
	assert.True(t, k.IsActive())
	assert.False(t, k.IsQuotaExceeded())
	assert.Equal(t, 0, k.RequestMade())
	assert.Nil(t, k.LastUsedAt())
	assert.Nil(t, k.QuotaResetTime())
	assert.Empty(t, k.LastError())
	assert.False(t, k.CreatedAt().IsZero())
	assert.False(t, k.UpdatedAt().IsZero())
}

func TestNewAPIKey_Invalid(t *testing.T) {
	tests := []struct {
		name     string
		service  string
		keyValue string
	}{
		{"empty service", "", "key-abc-123"},
		{"empty keyValue", "gemini", ""},
		{"both empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := entity.NewAPIKey(tt.service, tt.keyValue)
			require.Error(t, err)

			var domainErr *exception.InvalidAPIKeyException
			assert.True(t, errors.As(err, &domainErr))
		})
	}
}

func TestAPIKey_MarkAsUsed_IncrementsCounter(t *testing.T) {
	k, err := entity.NewAPIKey("gemini", "key-abc")
	require.NoError(t, err)

	k.MarkAsUsed()
	k.MarkAsUsed()
	k.MarkAsUsed()

	assert.Equal(t, 3, k.RequestMade())
	assert.NotNil(t, k.LastUsedAt())
}

func TestAPIKey_MarkAsQuotaExceeded_SetsFields(t *testing.T) {
	k, err := entity.NewAPIKey("gemini", "key-abc")
	require.NoError(t, err)

	resetTime := time.Now().Add(24 * time.Hour)
	k.MarkAsQuotaExceeded(resetTime, "quota_exhausted_rpd: some error")

	assert.True(t, k.IsQuotaExceeded())
	require.NotNil(t, k.QuotaResetTime())
	assert.WithinDuration(t, resetTime, *k.QuotaResetTime(), time.Second)
	assert.Equal(t, "quota_exhausted_rpd: some error", k.LastError())
}

func TestAPIKey_ResetQuota_ClearsFields(t *testing.T) {
	k, err := entity.NewAPIKey("gemini", "key-abc")
	require.NoError(t, err)

	k.MarkAsQuotaExceeded(time.Now().Add(24 * time.Hour), "")
	require.True(t, k.IsQuotaExceeded())

	k.ResetQuota()

	assert.False(t, k.IsQuotaExceeded())
	assert.Nil(t, k.QuotaResetTime())
}

func TestAPIKey_Deactivate_Activate(t *testing.T) {
	k, err := entity.NewAPIKey("gemini", "key-abc")
	require.NoError(t, err)

	require.True(t, k.IsActive())

	k.Deactivate()
	assert.False(t, k.IsActive())

	k.Activate()
	assert.True(t, k.IsActive())

	// Idempotent — deactivate twice
	k.Deactivate()
	k.Deactivate()
	assert.False(t, k.IsActive())
}

func TestRestoreAPIKey_Valid(t *testing.T) {
	now := time.Now()
	resetTime := now.Add(24 * time.Hour)

	k, err := entity.RestoreAPIKey(
		7, "gemini", "secret-key", "gemini",
		true, true, &resetTime,
		15, 1000000, 1500,
		42, &now, "some error",
		now, now,
	)

	require.NoError(t, err)
	assert.Equal(t, 7, k.ID())
	assert.Equal(t, "gemini", k.Service())
	assert.Equal(t, "secret-key", k.KeyValue())
	assert.True(t, k.IsActive())
	assert.True(t, k.IsQuotaExceeded())
	assert.Equal(t, 42, k.RequestMade())
	assert.Equal(t, "some error", k.LastError())
}

func TestRestoreAPIKey_EmptyService_ReturnsError(t *testing.T) {
	_, err := entity.RestoreAPIKey(1, "", "key", "gemini", false, false, nil, 15, 1000000, 1500, 0, nil, "", time.Now(), time.Now())
	require.Error(t, err)
}

func TestRestoreAPIKey_EmptyKeyValue_ReturnsError(t *testing.T) {
	_, err := entity.RestoreAPIKey(1, "gemini", "", "gemini", false, false, nil, 15, 1000000, 1500, 0, nil, "", time.Now(), time.Now())
	require.Error(t, err)
}
