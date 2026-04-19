package testmocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type MockAppSettingsRepository struct {
	mock.Mock
}

func (m *MockAppSettingsRepository) GetSetting(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}

func (m *MockAppSettingsRepository) SetSetting(ctx context.Context, key, value string) error {
	return m.Called(ctx, key, value).Error(0)
}
