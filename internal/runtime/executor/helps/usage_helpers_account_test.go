package helps

import (
	"context"
	"testing"

	sdkaccess "github.com/router-for-me/CLIProxyAPI/v7/sdk/access"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/usage"
)

func TestUsageReporterCopiesAccountMetadataFromContext(t *testing.T) {
	reporter := NewUsageReporter(context.Background(), "gemini", "gemini-2.5-pro", nil)
	ctx := sdkaccess.WithResult(context.Background(), &sdkaccess.Result{Metadata: map[string]string{
		sdkaccess.MetadataAccountID:   "team-a",
		sdkaccess.MetadataAccountName: "Team A",
		sdkaccess.MetadataAPIKeyID:    "ci",
		sdkaccess.MetadataAPIKeyName:  "CI",
	}})

	record := reporter.buildRecordFromContext(ctx, usage.Detail{InputTokens: 1}, false, usage.Failure{})
	if record.AccountID != "team-a" || record.AccountName != "Team A" || record.APIKeyID != "ci" || record.APIKeyName != "CI" {
		t.Fatalf("account usage fields = %#v", record)
	}
}
