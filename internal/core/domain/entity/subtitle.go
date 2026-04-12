package entity

import (
	"subsync/internal/core/domain/exception"
	"subsync/internal/core/domain/valueobject"
	"time"
)

type Subtitle struct {
	mediaInfo valueobject.MediaInfo
	status    valueobject.SubtitleStatus
	engPath   string
	lastError string
	embedded  bool
	createdAt time.Time
	updatedAt time.Time
}

func NewSubtitle(mediaInfo valueobject.MediaInfo, engPath string) (*Subtitle, error) {
	if engPath == "" {
		return nil, &exception.InvalidSubtitleException{Message: "engPath cannot be empty"}
	}
	return &Subtitle{
		mediaInfo: mediaInfo,
		status:    valueobject.StatusQueued,
		engPath:   engPath,
		createdAt: time.Now(),
		updatedAt: time.Now(),
	}, nil
}

func (s *Subtitle) EngPath() string {
	return s.engPath
}

func (s *Subtitle) Status() valueobject.SubtitleStatus {
	return s.status
}

func (s *Subtitle) LastError() string {
	return s.lastError
}

func (s *Subtitle) Embedded() bool {
	return s.embedded
}

func (s *Subtitle) CreatedAt() time.Time {
	return s.createdAt
}

func (s *Subtitle) UpdatedAt() time.Time {
	return s.updatedAt
}

func (s *Subtitle) TransitionTo(newStatus valueobject.SubtitleStatus) error {
	if s.status.CanTransitionTo(newStatus) {
		s.status = newStatus
		s.updatedAt = time.Now()
		return nil
	}
	return &exception.InvalidStatusTransitionException{From: string(s.status), To: string(newStatus)}

}

func (s *Subtitle) MarkEmbedded() {
	s.embedded = true
	s.updatedAt = time.Now()
}

func (s *Subtitle) MarkError(err error) {
	s.lastError = err.Error()
	s.updatedAt = time.Now()
}

func (s *Subtitle) MediaInfo() valueobject.MediaInfo {
	return s.mediaInfo
}