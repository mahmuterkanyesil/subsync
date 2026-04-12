package port

import "subsync/internal/core/domain/event"

// EventPublisher domain event'lerini yayınlamak için kullanılan port.
// nil geçilebilir — servisler publish öncesi nil kontrolü yapar.
type EventPublisher interface {
	Publish(e event.DomainEvent)
}
