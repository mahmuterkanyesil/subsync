package exception

import fmt "fmt"

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
type InvalidSubtitleStatusException struct {
	Message string
}

func (e *InvalidSubtitleStatusException) Error() string {
	return "invalid status transition from queued to embedded"
}

type InvalidStatusTransitionException struct {
    From string  // nereden
    To   string  // nereye
}

func (e *InvalidStatusTransitionException) Error() string {
	return fmt.Sprintf("invalid status transition from %s to %s", e.From, e.To)
}
