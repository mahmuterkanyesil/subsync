package testmocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	"subsync/internal/core/application/port"
)

type MockProgressStore struct {
	mock.Mock
}

func (m *MockProgressStore) Save(ctx context.Context, engPath string, blocks []port.SRTBlock) error {
	return m.Called(ctx, engPath, blocks).Error(0)
}

func (m *MockProgressStore) Load(ctx context.Context, engPath string) ([]port.SRTBlock, bool, error) {
	args := m.Called(ctx, engPath)
	if args.Get(0) == nil {
		return nil, args.Bool(1), args.Error(2)
	}
	return args.Get(0).([]port.SRTBlock), args.Bool(1), args.Error(2)
}

func (m *MockProgressStore) Clear(ctx context.Context, engPath string) error {
	return m.Called(ctx, engPath).Error(0)
}
