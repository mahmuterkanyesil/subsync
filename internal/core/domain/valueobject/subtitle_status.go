package valueobject

type SubtitleStatus string

const (
	StatusQueued         SubtitleStatus = "queued"
	StatusDone           SubtitleStatus = "done"
	StatusError          SubtitleStatus = "error"
	StatusQuotaExhausted SubtitleStatus = "quota_exhausted"
	StatusEmbedded       SubtitleStatus = "embedded"
	StatusEmbedFailed    SubtitleStatus = "embed_failed"
)

var validTransitions = map[SubtitleStatus][]SubtitleStatus{
	StatusQueued:         {StatusDone, StatusError, StatusQuotaExhausted},
	StatusDone:           {StatusEmbedded, StatusQueued, StatusEmbedFailed},
	StatusError:          {StatusQueued},
	StatusQuotaExhausted: {StatusQueued},
	StatusEmbedded:       {StatusDone},       // re-embed için
	StatusEmbedFailed:    {StatusDone},       // kalıcı hata düzeldikten sonra tekrar deneme için
}

func (s SubtitleStatus) CanTransitionTo(next SubtitleStatus) bool {
	allowedStatuses, isValid := validTransitions[s]
	if !isValid {
		return false
	}
	for _, status := range allowedStatuses {
		if status == next {
			return true
		}
	}
	return false
}
