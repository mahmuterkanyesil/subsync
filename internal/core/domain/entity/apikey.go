package entity

import (
	"subsync/internal/core/domain/exception"
	"time"
)

type APIKey struct {
	id       int
	service  string
	keyValue string

	isActive        bool
	isQuotaExceeded bool
	quotaResetTime  *time.Time

	requestMade int
	lastUsedAt  *time.Time
	lastError   string

	createdAt time.Time
	updatedAt time.Time
}

func NewAPIKey(service string, keyValue string) (*APIKey, error) {
	if service == "" {
		return nil, &exception.InvalidAPIKeyException{Message: "service cannot be empty"}
	}
	if keyValue == "" {
		return nil, &exception.InvalidAPIKeyException{Message: "keyValue cannot be empty"}
	}
	return &APIKey{
		service:   service,
		keyValue:  keyValue,
		createdAt: time.Now(),
		updatedAt: time.Now(),
	}, nil
}

func (a *APIKey) ID() int {
	return a.id
}

func (a *APIKey) Service() string {
	return a.service
}

func (a *APIKey) KeyValue() string {
	return a.keyValue
}

func (a *APIKey) IsActive() bool {
	return a.isActive
}

func (a *APIKey) IsQuotaExceeded() bool {
	return a.isQuotaExceeded
}

func (a *APIKey) QuotaResetTime() *time.Time {
	return a.quotaResetTime
}

func (a *APIKey) RequestMade() int {
	return a.requestMade
}

func (a *APIKey) LastUsedAt() *time.Time {
	return a.lastUsedAt
}

func (a *APIKey) LastError() string {
	return a.lastError
}

func (a *APIKey) CreatedAt() time.Time {
	return a.createdAt
}

func (a *APIKey) UpdatedAt() time.Time {
	return a.updatedAt
}

func (a *APIKey) MarkAsUsed() {
	a.requestMade++
	now := time.Now()
	a.lastUsedAt = &now
	a.updatedAt = now
}

func (a *APIKey) MarkAsQuotaExceeded(quotaResetTime time.Time) {
	a.isQuotaExceeded = true
	a.quotaResetTime = &quotaResetTime
	a.updatedAt = time.Now()
}

func (a *APIKey) ResetQuota() {
	a.isQuotaExceeded = false
	a.quotaResetTime = nil
	a.updatedAt = time.Now()
}

func (a *APIKey) Deactivate() {
	a.isActive = false
	a.updatedAt = time.Now()
}

func (a *APIKey) Activate() {
	a.isActive = true
	a.updatedAt = time.Now()
}
