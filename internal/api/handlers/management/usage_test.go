package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/redisqueue"
	coreusage "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
)

func TestGetUsageQueuePopsRequestedRecords(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withManagementUsageQueue(t, func() {
		redisqueue.Enqueue([]byte(`{"id":1}`))
		redisqueue.Enqueue([]byte(`{"id":2}`))
		redisqueue.Enqueue([]byte(`{"id":3}`))

		rec := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(rec)
		ginCtx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/usage-queue?count=2", nil)

		h := &Handler{}
		h.GetUsageQueue(ginCtx)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
		}

		var payload []json.RawMessage
		if errUnmarshal := json.Unmarshal(rec.Body.Bytes(), &payload); errUnmarshal != nil {
			t.Fatalf("unmarshal response: %v", errUnmarshal)
		}
		if len(payload) != 2 {
			t.Fatalf("response records = %d, want 2", len(payload))
		}
		requireRecordID(t, payload[0], 1)
		requireRecordID(t, payload[1], 2)

		remaining := redisqueue.PopOldest(10)
		if len(remaining) != 1 || string(remaining[0]) != `{"id":3}` {
			t.Fatalf("remaining queue = %q, want third item only", remaining)
		}
	})
}

func TestGetAccountUsageReturnsAccountSnapshots(t *testing.T) {
	gin.SetMode(gin.TestMode)
	redisqueue.ResetAccountUsageForTest()
	redisqueue.ResetUsageEventStoreForTest()
	t.Cleanup(redisqueue.ResetAccountUsageForTest)
	t.Cleanup(redisqueue.ResetUsageEventStoreForTest)

	usedAt := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	redisqueue.RecordAccountUsage(coreusage.Record{
		AccountID:   "team-a",
		AccountName: "Team A",
		APIKeyID:    "ci",
		APIKeyName:  "CI",
		RequestedAt: usedAt,
		Detail: coreusage.Detail{
			InputTokens:  11,
			OutputTokens: 13,
			TotalTokens:  24,
		},
	})

	rec := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(rec)
	ginCtx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/account-usage", nil)

	h := &Handler{}
	h.GetAccountUsage(ginCtx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload []redisqueue.AccountUsageSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(payload) != 1 {
		t.Fatalf("payload length = %d, want 1", len(payload))
	}
	got := payload[0]
	if got.AccountID != "team-a" || got.AccountName != "Team A" || got.APIKeyID != "ci" || got.APIKeyName != "CI" {
		t.Fatalf("identity = %#v", got)
	}
	if got.Requests != 1 || got.Failures != 0 || got.Tokens.TotalTokens != 24 || !got.LastUsedAt.Equal(usedAt) {
		t.Fatalf("snapshot = %#v", got)
	}
}

func TestGetAccountUsageReturnsPersistedAllTimeWhenAvailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	redisqueue.ResetAccountUsageForTest()
	redisqueue.ResetUsageEventStoreForTest()
	store, errStore := redisqueue.NewSQLiteUsageEventStore(filepath.Join(t.TempDir(), "usage.db"))
	if errStore != nil {
		t.Fatalf("NewSQLiteUsageEventStore: %v", errStore)
	}
	redisqueue.SetUsageEventStore(store)
	t.Cleanup(redisqueue.ResetAccountUsageForTest)
	t.Cleanup(redisqueue.ResetUsageEventStoreForTest)

	memoryOnlyAt := time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC)
	persistedAt := memoryOnlyAt.Add(time.Hour)
	redisqueue.RecordAccountUsage(coreusage.Record{
		AccountID:   "memory-only",
		AccountName: "Memory Only",
		APIKeyID:    "old",
		RequestedAt: memoryOnlyAt,
		Detail:      coreusage.Detail{TotalTokens: 100},
	})
	if errRecord := redisqueue.RecordAccountUsageEvent(context.Background(), coreusage.Record{
		AccountID:   "team-a",
		AccountName: "Team A",
		APIKeyID:    "ci",
		APIKeyName:  "CI",
		RequestedAt: persistedAt,
		Detail:      coreusage.Detail{TotalTokens: 24},
	}); errRecord != nil {
		t.Fatalf("RecordAccountUsageEvent: %v", errRecord)
	}

	rec := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(rec)
	ginCtx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/account-usage", nil)

	h := &Handler{}
	h.GetAccountUsage(ginCtx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload []redisqueue.AccountUsageSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(payload) != 1 {
		t.Fatalf("payload length = %d, want 1: %#v", len(payload), payload)
	}
	got := payload[0]
	if got.AccountID != "team-a" || got.APIKeyID != "ci" || got.Requests != 1 || got.Tokens.TotalTokens != 24 {
		t.Fatalf("persisted all-time snapshot = %#v", got)
	}
}

func TestGetAccountUsageReturnsRangeSnapshots(t *testing.T) {
	gin.SetMode(gin.TestMode)
	redisqueue.ResetUsageEventStoreForTest()
	store, errStore := redisqueue.NewSQLiteUsageEventStore(filepath.Join(t.TempDir(), "usage.db"))
	if errStore != nil {
		t.Fatalf("NewSQLiteUsageEventStore: %v", errStore)
	}
	redisqueue.SetUsageEventStore(store)
	t.Cleanup(redisqueue.ResetUsageEventStoreForTest)

	base := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)
	for _, record := range []coreusage.Record{
		{
			AccountID:   "team-a",
			AccountName: "Team A",
			APIKeyID:    "ci",
			APIKeyName:  "CI",
			RequestedAt: base.Add(-2 * time.Hour),
			Detail:      coreusage.Detail{InputTokens: 10, OutputTokens: 4, TotalTokens: 14},
		},
		{
			AccountID:   "team-a",
			AccountName: "Team A",
			APIKeyID:    "ci",
			APIKeyName:  "CI",
			RequestedAt: base.Add(time.Hour),
			Failed:      true,
			Detail:      coreusage.Detail{InputTokens: 3, OutputTokens: 2, ReasoningTokens: 1},
		},
	} {
		if errRecord := redisqueue.RecordAccountUsageEvent(context.Background(), record); errRecord != nil {
			t.Fatalf("RecordAccountUsageEvent: %v", errRecord)
		}
	}

	rec := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(rec)
	ginCtx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/account-usage?from=2026-05-18T11:00:00Z&to=2026-05-18T13:00:00Z", nil)

	h := &Handler{}
	h.GetAccountUsage(ginCtx)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var payload []redisqueue.AccountUsageSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(payload) != 1 {
		t.Fatalf("payload length = %d, want 1", len(payload))
	}
	got := payload[0]
	if got.Requests != 1 || got.Failures != 1 || got.Tokens.TotalTokens != 6 {
		t.Fatalf("range snapshot = %#v", got)
	}
	if !got.LastUsedAt.Equal(base.Add(time.Hour)) {
		t.Fatalf("LastUsedAt = %s, want %s", got.LastUsedAt, base.Add(time.Hour))
	}
}

func TestGetAccountUsageRejectsInvalidRange(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(rec)
	ginCtx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/account-usage?from=bad&to=2026-05-18T13:00:00Z", nil)

	h := &Handler{}
	h.GetAccountUsage(ginCtx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestGetAccountUsageRejectsInvertedRange(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(rec)
	ginCtx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/account-usage?from=2026-05-18T14:00:00Z&to=2026-05-18T13:00:00Z", nil)

	h := &Handler{}
	h.GetAccountUsage(ginCtx)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestGetUsageQueueInvalidCountDoesNotPop(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withManagementUsageQueue(t, func() {
		redisqueue.Enqueue([]byte(`{"id":1}`))

		rec := httptest.NewRecorder()
		ginCtx, _ := gin.CreateTestContext(rec)
		ginCtx.Request = httptest.NewRequest(http.MethodGet, "/v0/management/usage-queue?count=0", nil)

		h := &Handler{}
		h.GetUsageQueue(ginCtx)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
		}

		remaining := redisqueue.PopOldest(10)
		if len(remaining) != 1 || string(remaining[0]) != `{"id":1}` {
			t.Fatalf("remaining queue = %q, want original item", remaining)
		}
	})
}

func withManagementUsageQueue(t *testing.T, fn func()) {
	t.Helper()

	prevQueueEnabled := redisqueue.Enabled()
	redisqueue.SetEnabled(false)
	redisqueue.SetEnabled(true)

	defer func() {
		redisqueue.SetEnabled(false)
		redisqueue.SetEnabled(prevQueueEnabled)
	}()

	fn()
}

func requireRecordID(t *testing.T, raw json.RawMessage, want int) {
	t.Helper()

	var payload struct {
		ID int `json:"id"`
	}
	if errUnmarshal := json.Unmarshal(raw, &payload); errUnmarshal != nil {
		t.Fatalf("unmarshal record: %v", errUnmarshal)
	}
	if payload.ID != want {
		t.Fatalf("record id = %d, want %d", payload.ID, want)
	}
}
