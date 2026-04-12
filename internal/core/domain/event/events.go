package event

import "time"

// DomainEvent tüm domain event'lerinin uygulaması gereken temel interface.
type DomainEvent interface {
	EventName() string
	OccurredAt() time.Time
}

// TranslationCompleted bir altyazının Türkçeye çevrilip .tr.srt olarak yazıldığını bildirir.
type TranslationCompleted struct {
	EngPath    string
	occurredAt time.Time
}

func NewTranslationCompleted(engPath string) TranslationCompleted {
	return TranslationCompleted{EngPath: engPath, occurredAt: time.Now()}
}

func (e TranslationCompleted) EventName() string    { return "translation.completed" }
func (e TranslationCompleted) OccurredAt() time.Time { return e.occurredAt }

// EmbeddingCompleted bir altyazının video dosyasına embed edildiğini bildirir.
type EmbeddingCompleted struct {
	EngPath    string
	VideoPath  string
	occurredAt time.Time
}

func NewEmbeddingCompleted(engPath, videoPath string) EmbeddingCompleted {
	return EmbeddingCompleted{EngPath: engPath, VideoPath: videoPath, occurredAt: time.Now()}
}

func (e EmbeddingCompleted) EventName() string    { return "embedding.completed" }
func (e EmbeddingCompleted) OccurredAt() time.Time { return e.occurredAt }

// EmbeddingFailed kalıcı bir embed hatasını bildirir.
type EmbeddingFailed struct {
	EngPath    string
	Reason     string
	occurredAt time.Time
}

func NewEmbeddingFailed(engPath, reason string) EmbeddingFailed {
	return EmbeddingFailed{EngPath: engPath, Reason: reason, occurredAt: time.Now()}
}

func (e EmbeddingFailed) EventName() string    { return "embedding.failed" }
func (e EmbeddingFailed) OccurredAt() time.Time { return e.occurredAt }
