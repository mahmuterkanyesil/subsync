package valueobject

type SubtitleStatus string

const (
	StatusQueued         SubtitleStatus = "queued"
	StatusDone           SubtitleStatus = "done"
	StatusError          SubtitleStatus = "error"
	StatusQuotaExhausted SubtitleStatus = "quota_exhausted"
	StatusEmbedded       SubtitleStatus = "embedded"
)

var validTransitions = map[SubtitleStatus][]SubtitleStatus{
	StatusQueued:         {StatusDone, StatusError, StatusQuotaExhausted},
	StatusDone:           {StatusEmbedded, StatusQueued},
	StatusError:          {StatusQueued},
	StatusQuotaExhausted: {StatusQueued},
	StatusEmbedded:       {},
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
