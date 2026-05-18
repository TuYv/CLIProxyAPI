package redisqueue

import (
	"testing"
	"time"

	coreusage "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
)

func TestRecordAccountUsageAggregatesByAccountAndKey(t *testing.T) {
	ResetAccountUsageForTest()
	t.Cleanup(ResetAccountUsageForTest)

	first := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	second := first.Add(time.Minute)
	RecordAccountUsage(coreusage.Record{
		AccountID:   " team-a ",
		AccountName: " Team A ",
		APIKeyID:    " ci ",
		APIKeyName:  " CI ",
		RequestedAt: first,
		Detail: coreusage.Detail{
			InputTokens:  10,
			OutputTokens: 20,
			TotalTokens:  30,
		},
	})
	RecordAccountUsage(coreusage.Record{
		AccountID:   "team-a",
		AccountName: "Team A",
		APIKeyID:    "ci",
		APIKeyName:  "CI",
		RequestedAt: second,
		Failed:      true,
		Detail: coreusage.Detail{
			InputTokens:     5,
			OutputTokens:    7,
			ReasoningTokens: 3,
		},
	})
	RecordAccountUsage(coreusage.Record{
		AccountID: "team-b",
		APIKeyID:  "dev",
		Detail: coreusage.Detail{
			CachedTokens: 11,
		},
	})
	RecordAccountUsage(coreusage.Record{APIKeyID: "missing-account"})

	snapshots := AccountUsageSnapshots()
	if len(snapshots) != 2 {
		t.Fatalf("snapshots = %d, want 2: %#v", len(snapshots), snapshots)
	}

	teamA := snapshots[0]
	if teamA.AccountID != "team-a" || teamA.AccountName != "Team A" || teamA.APIKeyID != "ci" || teamA.APIKeyName != "CI" {
		t.Fatalf("teamA identity = %#v", teamA)
	}
	if teamA.Requests != 2 || teamA.Failures != 1 {
		t.Fatalf("teamA counters = requests:%d failures:%d, want requests:2 failures:1", teamA.Requests, teamA.Failures)
	}
	if teamA.Tokens.InputTokens != 15 || teamA.Tokens.OutputTokens != 27 || teamA.Tokens.ReasoningTokens != 3 || teamA.Tokens.TotalTokens != 45 {
		t.Fatalf("teamA tokens = %#v", teamA.Tokens)
	}
	if !teamA.LastUsedAt.Equal(second) {
		t.Fatalf("teamA LastUsedAt = %s, want %s", teamA.LastUsedAt, second)
	}

	teamB := snapshots[1]
	if teamB.AccountID != "team-b" || teamB.APIKeyID != "dev" || teamB.Requests != 1 || teamB.Tokens.TotalTokens != 11 {
		t.Fatalf("teamB snapshot = %#v", teamB)
	}
}
