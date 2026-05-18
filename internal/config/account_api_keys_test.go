package config

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestAccountAPIKeysYAMLConfig(t *testing.T) {
	raw := []byte(`
api-keys:
  - legacy-key
accounts:
  - id: team-a
    name: Team A
    metadata:
      owner: infra
    api-keys:
      - id: ci
        name: CI
        key: acct-key
        metadata:
          env: prod
`)

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
	if got := cfg.APIKeys; len(got) != 1 || got[0] != "legacy-key" {
		t.Fatalf("legacy APIKeys = %#v", got)
	}
	if len(cfg.Accounts) != 1 {
		t.Fatalf("Accounts length = %d, want 1", len(cfg.Accounts))
	}
	account := cfg.Accounts[0]
	if account.ID != "team-a" || account.Name != "Team A" {
		t.Fatalf("account identity = (%q, %q)", account.ID, account.Name)
	}
	if account.Metadata["owner"] != "infra" {
		t.Fatalf("account metadata owner = %q", account.Metadata["owner"])
	}
	if len(account.APIKeys) != 1 {
		t.Fatalf("account APIKeys length = %d, want 1", len(account.APIKeys))
	}
	key := account.APIKeys[0]
	if key.ID != "ci" || key.Name != "CI" || key.Key != "acct-key" {
		t.Fatalf("account key = %#v", key)
	}
	if key.Metadata["env"] != "prod" {
		t.Fatalf("key metadata env = %q", key.Metadata["env"])
	}
}

func TestAccountAPIKeysJSONConfig(t *testing.T) {
	raw := []byte(`{"accounts":[{"id":"team-a","api-keys":[{"id":"ci","key":"acct-key"}]}]}`)

	var cfg Config
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(cfg.Accounts) != 1 || cfg.Accounts[0].APIKeys[0].Key != "acct-key" {
		t.Fatalf("Accounts = %#v", cfg.Accounts)
	}
}
