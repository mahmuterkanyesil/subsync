package event

import (
	"subsync/internal/core/domain/event"
	"subsync/pkg/logger"
)

// LogEventPublisher publishes domain events by logging them.
// It satisfies port.EventPublisher and provides basic observability
// without requiring a message broker.
type LogEventPublisher struct{}

func NewLogEventPublisher() *LogEventPublisher {
	return &LogEventPublisher{}
}

func (p *LogEventPublisher) Publish(e event.DomainEvent) {
	logger.Info("event: %s occurred_at=%s", e.EventName(), e.OccurredAt().Format("2006-01-02T15:04:05Z"))
}
