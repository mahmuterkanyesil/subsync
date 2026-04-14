package testmocks

import (
	"github.com/stretchr/testify/mock"
	"subsync/internal/core/domain/event"
)

type MockEventPublisher struct {
	mock.Mock
}

func (m *MockEventPublisher) Publish(e event.DomainEvent) {
	m.Called(e)
}
