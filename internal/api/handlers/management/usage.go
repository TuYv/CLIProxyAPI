package management

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/redisqueue"
)

type usageQueueRecord []byte

func (r usageQueueRecord) MarshalJSON() ([]byte, error) {
	if json.Valid(r) {
		return append([]byte(nil), r...), nil
	}
	return json.Marshal(string(r))
}

// GetUsageQueue pops queued usage records from the usage queue.
func (h *Handler) GetUsageQueue(c *gin.Context) {
	if h == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler unavailable"})
		return
	}

	count, errCount := parseUsageQueueCount(c.Query("count"))
	if errCount != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errCount.Error()})
		return
	}

	items := redisqueue.PopOldest(count)
	records := make([]usageQueueRecord, 0, len(items))
	for _, item := range items {
		records = append(records, usageQueueRecord(append([]byte(nil), item...)))
	}

	c.JSON(http.StatusOK, records)
}

// GetAccountUsage returns aggregated usage grouped by client account API key.
func (h *Handler) GetAccountUsage(c *gin.Context) {
	if h == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler unavailable"})
		return
	}
	from, to, hasRange, errRange := parseAccountUsageRange(c.Query("from"), c.Query("to"))
	if errRange != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": errRange.Error()})
		return
	}
	if !hasRange {
		snapshots, errUsage := redisqueue.AccountUsageSnapshotsForRange(c.Request.Context(), nil, nil)
		if errUsage == nil {
			c.JSON(http.StatusOK, snapshots)
			return
		}
		if !errors.Is(errUsage, redisqueue.ErrUsageEventStoreUnavailable) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": errUsage.Error()})
			return
		}
		c.JSON(http.StatusOK, redisqueue.AccountUsageSnapshots())
		return
	}
	snapshots, errUsage := redisqueue.AccountUsageSnapshotsForRange(c.Request.Context(), from, to)
	if errUsage != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": errUsage.Error()})
		return
	}
	c.JSON(http.StatusOK, snapshots)
}

func parseUsageQueueCount(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 1, nil
	}
	count, errCount := strconv.Atoi(value)
	if errCount != nil || count <= 0 {
		return 0, errors.New("count must be a positive integer")
	}
	return count, nil
}

func parseAccountUsageRange(fromValue string, toValue string) (*time.Time, *time.Time, bool, error) {
	fromValue = strings.TrimSpace(fromValue)
	toValue = strings.TrimSpace(toValue)
	if fromValue == "" && toValue == "" {
		return nil, nil, false, nil
	}
	from, errFrom := parseOptionalUsageTime("from", fromValue)
	if errFrom != nil {
		return nil, nil, false, errFrom
	}
	to, errTo := parseOptionalUsageTime("to", toValue)
	if errTo != nil {
		return nil, nil, false, errTo
	}
	if from != nil && to != nil && from.After(*to) {
		return nil, nil, false, errors.New("from must be before or equal to to")
	}
	return from, to, true, nil
}

func parseOptionalUsageTime(name string, value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, errParse := time.Parse(time.RFC3339Nano, value)
	if errParse != nil {
		return nil, errors.New(name + " must be an RFC3339 timestamp")
	}
	parsed = parsed.UTC()
	return &parsed, nil
}
