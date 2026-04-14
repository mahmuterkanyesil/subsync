package testmocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type MockVideoProcessor struct {
	mock.Mock
}

func (m *MockVideoProcessor) EnsureEngSubtitle(ctx context.Context, videoPath string) (string, error) {
	args := m.Called(ctx, videoPath)
	return args.String(0), args.Error(1)
}

func (m *MockVideoProcessor) EmbedSubtitle(ctx context.Context, videoPath string, srtPath string) error {
	return m.Called(ctx, videoPath, srtPath).Error(0)
}

func (m *MockVideoProcessor) HasTurkishSubtitle(ctx context.Context, videoPath string) (bool, error) {
	args := m.Called(ctx, videoPath)
	return args.Bool(0), args.Error(1)
}
