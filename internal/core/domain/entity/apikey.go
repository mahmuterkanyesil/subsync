package entity

import (
	"subsync/internal/core/domain/exception"
	"time"
)

type APIKey struct {
	id       int
	service  string
	keyValue string
	model    string

	isActive        bool
	isQuotaExceeded bool
	quotaResetTime  *time.Time

	rpmLimit    int
	tpmLimit    int
	rpdLimit    int
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
		model:     service,
		isActive:  true,
		rpmLimit:  15,
		tpmLimit:  1000000,
		rpdLimit:  1500,
		createdAt: time.Now(),
		updatedAt: time.Now(),
	}, nil
}

func RestoreAPIKey(
	id int,
	service, keyValue, model string,
	isActive, isQuotaExceeded bool,
	quotaResetTime *time.Time,
	rpmLimit, tpmLimit, rpdLimit int,
	requestMade int,
	lastUsedAt *time.Time,
	lastError string,
	createdAt, updatedAt time.Time,
) (*APIKey, error) {
	if service == "" {
		return nil, &exception.InvalidAPIKeyException{Message: "service cannot be empty"}
	}
	if keyValue == "" {
		return nil, &exception.InvalidAPIKeyException{Message: "keyValue cannot be empty"}
	}
	return &APIKey{
		id:              id,
		service:         service,
		keyValue:        keyValue,
		model:           model,
		isActive:        isActive,
		isQuotaExceeded: isQuotaExceeded,
		quotaResetTime:  quotaResetTime,
		rpmLimit:        rpmLimit,
		tpmLimit:        tpmLimit,
		rpdLimit:        rpdLimit,
		requestMade:     requestMade,
		lastUsedAt:      lastUsedAt,
		lastError:       lastError,
		createdAt:       createdAt,
		updatedAt:       updatedAt,
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

func (a *APIKey) Model() string {
	return a.model
}

func (a *APIKey) RPMLimit() int {
	return a.rpmLimit
}

func (a *APIKey) TPMLimit() int {
	return a.tpmLimit
}

func (a *APIKey) RPDLimit() int {
	return a.rpdLimit
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
