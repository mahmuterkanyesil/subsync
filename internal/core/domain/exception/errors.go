package exception

import "fmt"

type DomainException struct {
	Message string
}

func (e *DomainException) Error() string {
	return e.Message
}

type InvalidMediaInfoException struct {
	Message string
}

func (e *InvalidMediaInfoException) Error() string {
	return e.Message
}

type InvalidMediaTypeException struct {
	Message string
}

func (e *InvalidMediaTypeException) Error() string {
	return e.Message
}

type InvalidSubtitleException struct {
	Message string
}

func (e *InvalidSubtitleException) Error() string {
	return e.Message
}

type InvalidAPIKeyException struct {
	Message string
}

func (e *InvalidAPIKeyException) Error() string {
	return e.Message
}

// InvalidStatusTransitionException geçersiz durum geçişlerini temsil eder.
type InvalidStatusTransitionException struct {
	From string
	To   string
}

func (e *InvalidStatusTransitionException) Error() string {
	return fmt.Sprintf("invalid status transition from %s to %s", e.From, e.To)
}
