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

func (m *MockSubtitleRepository) Statistics(ctx context.Context) (*port.SubtitleStats, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*port.SubtitleStats), args.Error(1)
}

func (m *MockSubtitleRepository) FindByStatus(ctx context.Context, status valueobject.SubtitleStatus) ([]*entity.Subtitle, error) {
	args := m.Called(ctx, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*entity.Subtitle), args.Error(1)
}

func (m *MockSubtitleRepository) Delete(ctx context.Context, engPath string) error {
	return m.Called(ctx, engPath).Error(0)
}

func (m *MockSubtitleRepository) DeleteMany(ctx context.Context, engPaths []string) error {
	return m.Called(ctx, engPaths).Error(0)
}

func (m *MockSubtitleRepository) FindWithFilters(ctx context.Context, f port.SubtitleFilter) (*port.SubtitlePage, error) {
	args := m.Called(ctx, f)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*port.SubtitlePage), args.Error(1)
}
