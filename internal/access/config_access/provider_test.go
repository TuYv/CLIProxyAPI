package configaccess

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	sdkaccess "github.com/router-for-me/CLIProxyAPI/v7/sdk/access"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v7/sdk/config"
)

func TestProviderAuthenticatesAccountAPIKeyMetadata(t *testing.T) {
	p := newProvider(sdkaccess.DefaultAccessProviderName, []keyRecord{{
		Key:         "acct-key",
		AccountID:   "team-a",
		AccountName: "Team A",
		APIKeyID:    "ci",
		APIKeyName:  "CI",
		Metadata:    map[string]string{"owner": "infra"},
	}})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer acct-key")
	res, authErr := p.Authenticate(context.Background(), req)
	if authErr != nil {
		t.Fatalf("Authenticate() error = %v", authErr)
	}
	if res.Principal != "acct-key" {
		t.Fatalf("Principal = %q", res.Principal)
	}
	want := map[string]string{
		"account_id":   "team-a",
		"account_name": "Team A",
		"api_key_id":   "ci",
		"api_key_name": "CI",
		"owner":        "infra",
		"source":       "authorization",
	}
	for key, value := range want {
		if res.Metadata[key] != value {
			t.Fatalf("Metadata[%q] = %q, want %q; all metadata=%#v", key, res.Metadata[key], value, res.Metadata)
		}
	}
}

func TestAccountRecordsFromConfigSkipDisabledAccountsAndKeys(t *testing.T) {
	cfg := &sdkconfig.SDKConfig{
		APIKeys: []string{"legacy-key"},
		Accounts: []sdkconfig.ClientAccount{
			{ID: "disabled-account", Disabled: true, APIKeys: []sdkconfig.ClientAPIKey{{ID: "k1", Key: "disabled-account-key"}}},
			{ID: "team-a", APIKeys: []sdkconfig.ClientAPIKey{{ID: "disabled-key", Key: "disabled-key", Disabled: true}, {ID: "ci", Key: "acct-key"}}},
		},
	}
	records := accountRecordsFromConfig(cfg)
	if len(records) != 2 {
		t.Fatalf("records length = %d, want 2: %#v", len(records), records)
	}
	if records[0].Key != "acct-key" || records[0].AccountID != "team-a" || records[0].APIKeyID != "ci" {
		t.Fatalf("first record = %#v", records[0])
	}
	if records[1].Key != "legacy-key" || records[1].AccountID != "default" || records[1].APIKeyID != "legacy-key" {
		t.Fatalf("legacy record = %#v", records[1])
	}
}

func TestAccountRecordsFirstAccountKeyWinsDuplicate(t *testing.T) {
	cfg := &sdkconfig.SDKConfig{
		APIKeys: []string{"same-key"},
		Accounts: []sdkconfig.ClientAccount{
			{ID: "team-a", APIKeys: []sdkconfig.ClientAPIKey{{ID: "a", Key: "same-key"}}},
			{ID: "team-b", APIKeys: []sdkconfig.ClientAPIKey{{ID: "b", Key: "same-key"}}},
		},
	}
	records := accountRecordsFromConfig(cfg)
	if len(records) != 1 {
		t.Fatalf("records length = %d, want 1: %#v", len(records), records)
	}
	if records[0].AccountID != "team-a" || records[0].APIKeyID != "a" {
		t.Fatalf("duplicate winner = %#v", records[0])
	}
}
