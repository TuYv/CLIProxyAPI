package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	clientaccess "github.com/router-for-me/CLIProxyAPI/v7/sdk/access"
	coreexecutor "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/executor"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v7/sdk/config"
)

func TestRequestExecutionMetadataIncludesExecutionSessionWithoutIdempotencyKey(t *testing.T) {
	ctx := WithExecutionSessionID(context.Background(), "session-1")

	meta := requestExecutionMetadata(ctx)
	if got := meta[coreexecutor.ExecutionSessionMetadataKey]; got != "session-1" {
		t.Fatalf("ExecutionSessionMetadataKey = %v, want %q", got, "session-1")
	}
	if _, ok := meta[idempotencyKeyMetadataKey]; ok {
		t.Fatalf("unexpected idempotency key in metadata: %v", meta[idempotencyKeyMetadataKey])
	}
}

func TestGetContextWithCancelPreservesClientAccessResult(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	ginCtx, _ := gin.CreateTestContext(rec)
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	request = request.WithContext(clientaccess.WithResult(request.Context(), &clientaccess.Result{
		Provider:  "config-api-key",
		Principal: "client-key",
		Metadata: map[string]string{
			clientaccess.MetadataAccountID: "rick",
			clientaccess.MetadataAPIKeyID:  "key-1",
		},
	}))
	ginCtx.Request = request

	handler := NewBaseAPIHandlers(&sdkconfig.SDKConfig{}, nil)
	ctx, cancel := handler.GetContextWithCancel(nil, ginCtx, context.Background())
	defer cancel()

	result, ok := clientaccess.ResultFromContext(ctx)
	if !ok {
		t.Fatal("expected client access result in execution context")
	}
	if result.Metadata[clientaccess.MetadataAccountID] != "rick" || result.Metadata[clientaccess.MetadataAPIKeyID] != "key-1" {
		t.Fatalf("metadata = %#v", result.Metadata)
	}
}

func TestSetReasoningEffortMetadataUsesSuffixOverBody(t *testing.T) {
	meta := make(map[string]any)

	setReasoningEffortMetadata(meta, "openai", "gpt-5.4(high)", []byte(`{"reasoning_effort":"low"}`))

	if got := meta[coreexecutor.ReasoningEffortMetadataKey]; got != "high" {
		t.Fatalf("ReasoningEffortMetadataKey = %v, want %q", got, "high")
	}
}

func TestSetReasoningEffortMetadataSupportsOpenAIResponses(t *testing.T) {
	meta := make(map[string]any)

	setReasoningEffortMetadata(meta, "openai-response", "gpt-5.4", []byte(`{"reasoning":{"effort":"medium"}}`))

	if got := meta[coreexecutor.ReasoningEffortMetadataKey]; got != "medium" {
		t.Fatalf("ReasoningEffortMetadataKey = %v, want %q", got, "medium")
	}
}

func TestSetServiceTierMetadataExtractsValue(t *testing.T) {
	meta := make(map[string]any)

	setServiceTierMetadata(meta, []byte(`{"service_tier":"priority"}`))

	gotServiceTier := meta[coreexecutor.ServiceTierMetadataKey]
	if gotServiceTier != "priority" {
		t.Fatalf("ServiceTierMetadataKey = %v, want %q", gotServiceTier, "priority")
	}
}

func TestSetServiceTierMetadataDefaultsWhenMissing(t *testing.T) {
	meta := make(map[string]any)

	setServiceTierMetadata(meta, []byte(`{"model":"gpt-5.4"}`))

	gotServiceTier := meta[coreexecutor.ServiceTierMetadataKey]
	if gotServiceTier != "default" {
		t.Fatalf("ServiceTierMetadataKey = %v, want %q", gotServiceTier, "default")
	}
}
