package testmocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	"subsync/internal/core/application/port"
	"subsync/internal/core/domain/entity"
	"subsync/internal/core/domain/valueobject"
)

type MockSubtitleRepository struct {
	mock.Mock
}

func (m *MockSubtitleRepository) Save(ctx context.Context, s *entity.Subtitle) error {
	return m.Called(ctx, s).Error(0)
}

func (m *MockSubtitleRepository) FindByPath(ctx context.Context, path string) (*entity.Subtitle, error) {
	args := m.Called(ctx, path)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Subtitle), args.Error(1)
}

func (m *MockSubtitleRepository) FindAll(ctx context.Context) ([]*entity.Subtitle, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entity.Subtitle), args.Error(1)
}

func (m *MockSubtitleRepository) FindPendingEmbed(ctx context.Context) ([]*entity.Subtitle, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entity.Subtitle), args.Error(1)
}

func (m *MockSubtitleRepository) Statistics(ctx context.Context) (port.SubtitleStats, error) {
	args := m.Called(ctx)
	return args.Get(0).(port.SubtitleStats), args.Error(1)
}

func (m *MockSubtitleRepository) FindBySxxExx(ctx context.Context, season, episode int) ([]*entity.Subtitle, error) {
	args := m.Called(ctx, season, episode)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entity.Subtitle), args.Error(1)
}

func (m *MockSubtitleRepository) FindByStatus(ctx context.Context, status valueobject.SubtitleStatus) ([]*entity.Subtitle, error) {
	args := m.Called(ctx, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entity.Subtitle), args.Error(1)
}
