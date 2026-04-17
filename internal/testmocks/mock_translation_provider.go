package testmocks

import (
	"context"

	"github.com/stretchr/testify/mock"
	"subsync/internal/core/application/port"
)

type MockTranslationProvider struct {
	mock.Mock
}

func (m *MockTranslationProvider) TranslateBatch(ctx context.Context, blocks []port.SRTBlock, keyValue, model string) ([]port.SRTBlock, error) {
	args := m.Called(ctx, blocks, keyValue, model)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]port.SRTBlock), args.Error(1)
}
