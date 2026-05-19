package redisqueue

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type sqliteUsageEventStore struct {
	db *sql.DB
}

func NewSQLiteUsageEventStore(path string) (UsageEventStore, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("usage sqlite path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create usage sqlite directory: %w", err)
	}
	db, errOpen := sql.Open("sqlite", path)
	if errOpen != nil {
		return nil, fmt.Errorf("open usage sqlite store: %w", errOpen)
	}
	store := &sqliteUsageEventStore{db: db}
	if errInit := store.init(context.Background()); errInit != nil {
		_ = db.Close()
		return nil, errInit
	}
	return store, nil
}

func (s *sqliteUsageEventStore) init(ctx context.Context) error {
	statements := []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA busy_timeout=5000`,
		`CREATE TABLE IF NOT EXISTS usage_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			requested_at INTEGER NOT NULL,
			account_id TEXT NOT NULL,
			account_name TEXT,
			api_key_id TEXT NOT NULL,
			api_key_name TEXT,
			requests INTEGER NOT NULL,
			failures INTEGER NOT NULL,
			input_tokens INTEGER NOT NULL,
			output_tokens INTEGER NOT NULL,
			reasoning_tokens INTEGER NOT NULL,
			cached_tokens INTEGER NOT NULL,
			cache_read_tokens INTEGER NOT NULL,
			cache_creation_tokens INTEGER NOT NULL,
			total_tokens INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_time ON usage_events(requested_at)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_account_time ON usage_events(account_id, requested_at)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_events_key_time ON usage_events(account_id, api_key_id, requested_at)`,
	}
	for _, statement := range statements {
		if _, errExec := s.db.ExecContext(ctx, statement); errExec != nil {
			return fmt.Errorf("initialize usage sqlite store: %w", errExec)
		}
	}
	return nil
}

func (s *sqliteUsageEventStore) Append(ctx context.Context, event UsageEvent) error {
	if s == nil || s.db == nil {
		return ErrUsageEventStoreUnavailable
	}
	if strings.TrimSpace(event.AccountID) == "" {
		return nil
	}
	if event.RequestedAt.IsZero() {
		event.RequestedAt = time.Now()
	}
	_, errExec := s.db.ExecContext(ctx, `INSERT INTO usage_events (
		requested_at, account_id, account_name, api_key_id, api_key_name, requests, failures,
		input_tokens, output_tokens, reasoning_tokens, cached_tokens, cache_read_tokens,
		cache_creation_tokens, total_tokens
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.RequestedAt.UTC().UnixNano(),
		strings.TrimSpace(event.AccountID),
		strings.TrimSpace(event.AccountName),
		strings.TrimSpace(event.APIKeyID),
		strings.TrimSpace(event.APIKeyName),
		event.Requests,
		event.Failures,
		event.Tokens.InputTokens,
		event.Tokens.OutputTokens,
		event.Tokens.ReasoningTokens,
		event.Tokens.CachedTokens,
		event.Tokens.CacheReadTokens,
		event.Tokens.CacheCreationTokens,
		event.Tokens.TotalTokens,
	)
	if errExec != nil {
		return fmt.Errorf("append usage event: %w", errExec)
	}
	return nil
}

func (s *sqliteUsageEventStore) AccountUsage(ctx context.Context, from *time.Time, to *time.Time) ([]AccountUsageSnapshot, error) {
	if s == nil || s.db == nil {
		return nil, ErrUsageEventStoreUnavailable
	}
	where := make([]string, 0, 2)
	args := make([]any, 0, 2)
	if from != nil {
		where = append(where, "requested_at >= ?")
		args = append(args, from.UTC().UnixNano())
	}
	if to != nil {
		where = append(where, "requested_at <= ?")
		args = append(args, to.UTC().UnixNano())
	}
	query := `SELECT
		account_id,
		MAX(account_name),
		api_key_id,
		MAX(api_key_name),
		SUM(requests),
		SUM(failures),
		SUM(input_tokens),
		SUM(output_tokens),
		SUM(reasoning_tokens),
		SUM(cached_tokens),
		SUM(cache_read_tokens),
		SUM(cache_creation_tokens),
		SUM(total_tokens),
		MAX(requested_at)
	FROM usage_events`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " GROUP BY account_id, api_key_id"

	rows, errQuery := s.db.QueryContext(ctx, query, args...)
	if errQuery != nil {
		return nil, fmt.Errorf("query account usage events: %w", errQuery)
	}
	defer func() { _ = rows.Close() }()

	snapshots := make([]AccountUsageSnapshot, 0)
	for rows.Next() {
		var snapshot AccountUsageSnapshot
		var lastUsedAt int64
		if errScan := rows.Scan(
			&snapshot.AccountID,
			&snapshot.AccountName,
			&snapshot.APIKeyID,
			&snapshot.APIKeyName,
			&snapshot.Requests,
			&snapshot.Failures,
			&snapshot.Tokens.InputTokens,
			&snapshot.Tokens.OutputTokens,
			&snapshot.Tokens.ReasoningTokens,
			&snapshot.Tokens.CachedTokens,
			&snapshot.Tokens.CacheReadTokens,
			&snapshot.Tokens.CacheCreationTokens,
			&snapshot.Tokens.TotalTokens,
			&lastUsedAt,
		); errScan != nil {
			return nil, fmt.Errorf("scan account usage events: %w", errScan)
		}
		snapshot.LastUsedAt = time.Unix(0, lastUsedAt).UTC()
		snapshots = append(snapshots, snapshot)
	}
	if errRows := rows.Err(); errRows != nil {
		return nil, fmt.Errorf("iterate account usage events: %w", errRows)
	}
	sortAccountUsageSnapshots(snapshots)
	return snapshots, nil
}

func (s *sqliteUsageEventStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func sortAccountUsageSnapshots(snapshots []AccountUsageSnapshot) {
	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].AccountID != snapshots[j].AccountID {
			return snapshots[i].AccountID < snapshots[j].AccountID
		}
		return snapshots[i].APIKeyID < snapshots[j].APIKeyID
	})
}
