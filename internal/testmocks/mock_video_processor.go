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

func (m *MockVideoProcessor) EmbedSubtitle(ctx context.Context, videoPath, srtPath, langFFmpegCode, langNameEN string) error {
	return m.Called(ctx, videoPath, srtPath, langFFmpegCode, langNameEN).Error(0)
}

func (m *MockVideoProcessor) HasTargetSubtitle(ctx context.Context, videoPath, langFFmpegCode string) (bool, error) {
	args := m.Called(ctx, videoPath, langFFmpegCode)
	return args.Bool(0), args.Error(1)
}
