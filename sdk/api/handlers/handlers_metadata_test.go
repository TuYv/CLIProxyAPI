package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	clientaccess "github.com/router-for-me/CLIProxyAPI/v7/sdk/access"
	coreexecutor "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/executor"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v7/sdk/config"
	"golang.org/x/net/context"
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
