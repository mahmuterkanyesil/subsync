package testmocks

import (
	"context"
	"time"

	"github.com/stretchr/testify/mock"
	"subsync/internal/core/application/port"
	"subsync/internal/core/domain/entity"
)

type MockAPIKeyRepository struct {
	mock.Mock
}

func (m *MockAPIKeyRepository) Save(ctx context.Context, k *entity.APIKey) error {
	return m.Called(ctx, k).Error(0)
}

func (m *MockAPIKeyRepository) FindByID(ctx context.Context, id int) (*entity.APIKey, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepository) FindNextAvailable(ctx context.Context, service string) (*entity.APIKey, error) {
	args := m.Called(ctx, service)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepository) ResetExpiredQuotas(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

func (m *MockAPIKeyRepository) FindAll(ctx context.Context) ([]*entity.APIKey, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entity.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepository) FindEarliestQuotaReset(ctx context.Context, service string) (*time.Time, error) {
	args := m.Called(ctx, service)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*time.Time), args.Error(1)
}

func (m *MockAPIKeyRepository) Delete(ctx context.Context, id int) error {
	return m.Called(ctx, id).Error(0)
}

func (m *MockAPIKeyRepository) IncrementModelUsage(ctx context.Context, keyID int, model string) error {
	return m.Called(ctx, keyID, model).Error(0)
}

func (m *MockAPIKeyRepository) FindAllModelUsage(ctx context.Context, keyID int) ([]port.ModelUsage, error) {
	args := m.Called(ctx, keyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]port.ModelUsage), args.Error(1)
}

func (m *MockAPIKeyRepository) ResetExpiredModelUsages(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}
