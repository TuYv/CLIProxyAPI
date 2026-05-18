package redisqueue

import (
	"sort"
	"strings"
	"sync"
	"time"

	coreusage "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
)

type accountUsageKey struct {
	accountID string
	apiKeyID  string
}

// AccountUsageSnapshot contains aggregated usage for one account API key.
type AccountUsageSnapshot struct {
	AccountID   string     `json:"account_id"`
	AccountName string     `json:"account_name,omitempty"`
	APIKeyID    string     `json:"api_key_id"`
	APIKeyName  string     `json:"api_key_name,omitempty"`
	Requests    int64      `json:"requests"`
	Failures    int64      `json:"failures"`
	Tokens      tokenStats `json:"tokens"`
	LastUsedAt  time.Time  `json:"last_used_at"`
}

var accountUsageStore = struct {
	sync.RWMutex
	records map[accountUsageKey]AccountUsageSnapshot
}{records: make(map[accountUsageKey]AccountUsageSnapshot)}

// RecordAccountUsage aggregates usage by client account and account API key.
func RecordAccountUsage(record coreusage.Record) {
	accountID := strings.TrimSpace(record.AccountID)
	if accountID == "" {
		return
	}
	apiKeyID := strings.TrimSpace(record.APIKeyID)
	key := accountUsageKey{accountID: accountID, apiKeyID: apiKeyID}
	timestamp := record.RequestedAt
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	accountUsageStore.Lock()
	defer accountUsageStore.Unlock()

	snapshot := accountUsageStore.records[key]
	snapshot.AccountID = accountID
	snapshot.AccountName = strings.TrimSpace(record.AccountName)
	snapshot.APIKeyID = apiKeyID
	snapshot.APIKeyName = strings.TrimSpace(record.APIKeyName)
	snapshot.Requests++
	if record.Failed {
		snapshot.Failures++
	}
	snapshot.Tokens.add(tokensFromDetail(record.Detail))
	if timestamp.After(snapshot.LastUsedAt) || snapshot.LastUsedAt.IsZero() {
		snapshot.LastUsedAt = timestamp
	}
	accountUsageStore.records[key] = snapshot
}

// AccountUsageSnapshots returns a stable copy of account API key usage aggregates.
func AccountUsageSnapshots() []AccountUsageSnapshot {
	accountUsageStore.RLock()
	defer accountUsageStore.RUnlock()

	snapshots := make([]AccountUsageSnapshot, 0, len(accountUsageStore.records))
	for _, snapshot := range accountUsageStore.records {
		snapshots = append(snapshots, snapshot)
	}
	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].AccountID != snapshots[j].AccountID {
			return snapshots[i].AccountID < snapshots[j].AccountID
		}
		return snapshots[i].APIKeyID < snapshots[j].APIKeyID
	})
	return snapshots
}

func ResetAccountUsageForTest() {
	accountUsageStore.Lock()
	accountUsageStore.records = make(map[accountUsageKey]AccountUsageSnapshot)
	accountUsageStore.Unlock()
}

func (t *tokenStats) add(other tokenStats) {
	t.InputTokens += other.InputTokens
	t.OutputTokens += other.OutputTokens
	t.ReasoningTokens += other.ReasoningTokens
	t.CachedTokens += other.CachedTokens
	t.CacheReadTokens += other.CacheReadTokens
	t.CacheCreationTokens += other.CacheCreationTokens
	t.TotalTokens += other.TotalTokens
}
