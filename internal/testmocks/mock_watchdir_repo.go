package testmocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	"subsync/internal/core/domain/entity"
)

type MockWatchDirRepository struct {
	mock.Mock
}

func (m *MockWatchDirRepository) FindAll(ctx context.Context) ([]*entity.WatchDir, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entity.WatchDir), args.Error(1)
}

func (m *MockWatchDirRepository) FindEnabled(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockWatchDirRepository) FindByID(ctx context.Context, id int) (*entity.WatchDir, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.WatchDir), args.Error(1)
}

func (m *MockWatchDirRepository) Save(ctx context.Context, w *entity.WatchDir) error {
	return m.Called(ctx, w).Error(0)
}

func (m *MockWatchDirRepository) Delete(ctx context.Context, id int) error {
	return m.Called(ctx, id).Error(0)
}
