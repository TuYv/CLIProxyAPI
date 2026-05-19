package redisqueue

import (
	"context"
	"strings"
	"sync"
	"time"

	coreusage "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
)

type UsageEvent struct {
	RequestedAt time.Time
	AccountID   string
	AccountName string
	APIKeyID    string
	APIKeyName  string
	Requests    int64
	Failures    int64
	Tokens      tokenStats
}

type UsageEventStore interface {
	Append(ctx context.Context, event UsageEvent) error
	AccountUsage(ctx context.Context, from *time.Time, to *time.Time) ([]AccountUsageSnapshot, error)
	Close() error
}

var usageEvents = struct {
	sync.RWMutex
	store UsageEventStore
}{}

func SetUsageEventStore(store UsageEventStore) {
	usageEvents.Lock()
	old := usageEvents.store
	usageEvents.store = store
	usageEvents.Unlock()
	if old != nil && old != store {
		_ = old.Close()
	}
}

func ResetUsageEventStoreForTest() {
	usageEvents.Lock()
	store := usageEvents.store
	usageEvents.store = nil
	usageEvents.Unlock()
	if store != nil {
		_ = store.Close()
	}
}

func RecordAccountUsageEvent(ctx context.Context, record coreusage.Record) error {
	event, ok := usageEventFromRecord(record)
	if !ok {
		return nil
	}
	usageEvents.RLock()
	store := usageEvents.store
	usageEvents.RUnlock()
	if store == nil {
		return nil
	}
	return store.Append(ctx, event)
}

func AccountUsageSnapshotsForRange(ctx context.Context, from *time.Time, to *time.Time) ([]AccountUsageSnapshot, error) {
	usageEvents.RLock()
	store := usageEvents.store
	usageEvents.RUnlock()
	if store == nil {
		return nil, ErrUsageEventStoreUnavailable
	}
	return store.AccountUsage(ctx, from, to)
}

func usageEventFromRecord(record coreusage.Record) (UsageEvent, bool) {
	accountID := stringsTrim(record.AccountID)
	if accountID == "" {
		return UsageEvent{}, false
	}
	timestamp := record.RequestedAt
	if timestamp.IsZero() {
		timestamp = time.Now()
	}
	event := UsageEvent{
		RequestedAt: timestamp.UTC(),
		AccountID:   accountID,
		AccountName: stringsTrim(record.AccountName),
		APIKeyID:    stringsTrim(record.APIKeyID),
		APIKeyName:  stringsTrim(record.APIKeyName),
		Requests:    1,
		Tokens:      tokensFromDetail(record.Detail),
	}
	if record.Failed {
		event.Failures = 1
	}
	return event, true
}

func stringsTrim(value string) string {
	return strings.TrimSpace(value)
}
