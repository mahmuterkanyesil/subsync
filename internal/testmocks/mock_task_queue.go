package testmocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type MockTaskQueue struct {
	mock.Mock
}

func (m *MockTaskQueue) Enqueue(ctx context.Context, taskName string, payload any) error {
	return m.Called(ctx, taskName, payload).Error(0)
}
