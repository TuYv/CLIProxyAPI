package configaccess

import (
	"context"
	"net/http"
	"strings"

	sdkaccess "github.com/router-for-me/CLIProxyAPI/v7/sdk/access"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v7/sdk/config"
)

// Register ensures the config-access provider is available to the access manager.
func Register(cfg *sdkconfig.SDKConfig) {
	if cfg == nil {
		sdkaccess.UnregisterProvider(sdkaccess.AccessProviderTypeConfigAPIKey)
		return
	}

	records := accountRecordsFromConfig(cfg)
	if len(records) == 0 {
		sdkaccess.UnregisterProvider(sdkaccess.AccessProviderTypeConfigAPIKey)
		return
	}

	sdkaccess.RegisterProvider(
		sdkaccess.AccessProviderTypeConfigAPIKey,
		newProvider(sdkaccess.DefaultAccessProviderName, records),
	)
}

type keyRecord struct {
	Key         string
	AccountID   string
	AccountName string
	APIKeyID    string
	APIKeyName  string
	Metadata    map[string]string
}

type provider struct {
	name string
	keys map[string]keyRecord
}

func newProvider(name string, records []keyRecord) *provider {
	providerName := strings.TrimSpace(name)
	if providerName == "" {
		providerName = sdkaccess.DefaultAccessProviderName
	}
	keySet := make(map[string]keyRecord, len(records))
	for _, record := range records {
		key := strings.TrimSpace(record.Key)
		if key == "" {
			continue
		}
		record.Key = key
		if _, exists := keySet[key]; exists {
			continue
		}
		keySet[key] = record
	}
	return &provider{name: providerName, keys: keySet}
}

func (p *provider) Identifier() string {
	if p == nil || p.name == "" {
		return sdkaccess.DefaultAccessProviderName
	}
	return p.name
}

func (p *provider) Authenticate(_ context.Context, r *http.Request) (*sdkaccess.Result, *sdkaccess.AuthError) {
	if p == nil {
		return nil, sdkaccess.NewNotHandledError()
	}
	if len(p.keys) == 0 {
		return nil, sdkaccess.NewNotHandledError()
	}
	authHeader := r.Header.Get("Authorization")
	authHeaderGoogle := r.Header.Get("X-Goog-Api-Key")
	authHeaderAnthropic := r.Header.Get("X-Api-Key")
	queryKey := ""
	queryAuthToken := ""
	if r.URL != nil {
		queryKey = r.URL.Query().Get("key")
		queryAuthToken = r.URL.Query().Get("auth_token")
	}
	if authHeader == "" && authHeaderGoogle == "" && authHeaderAnthropic == "" && queryKey == "" && queryAuthToken == "" {
		return nil, sdkaccess.NewNoCredentialsError()
	}

	apiKey := extractBearerToken(authHeader)

	candidates := []struct {
		value  string
		source string
	}{
		{apiKey, "authorization"},
		{authHeaderGoogle, "x-goog-api-key"},
		{authHeaderAnthropic, "x-api-key"},
		{queryKey, "query-key"},
		{queryAuthToken, "query-auth-token"},
	}

	for _, candidate := range candidates {
		if candidate.value == "" {
			continue
		}
		if record, ok := p.keys[candidate.value]; ok {
			metadata := metadataFromKeyRecord(record)
			metadata["source"] = candidate.source
			return &sdkaccess.Result{
				Provider:  p.Identifier(),
				Principal: candidate.value,
				Metadata:  metadata,
			}, nil
		}
	}

	return nil, sdkaccess.NewInvalidCredentialError()
}

func extractBearerToken(header string) string {
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return header
	}
	if strings.ToLower(parts[0]) != "bearer" {
		return header
	}
	return strings.TrimSpace(parts[1])
}

func normalizeKeys(keys []string) []string {
	if len(keys) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		if _, exists := seen[trimmedKey]; exists {
			continue
		}
		seen[trimmedKey] = struct{}{}
		normalized = append(normalized, trimmedKey)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func accountRecordsFromConfig(cfg *sdkconfig.SDKConfig) []keyRecord {
	if cfg == nil {
		return nil
	}
	records := make([]keyRecord, 0)
	seen := make(map[string]struct{})
	for _, account := range cfg.Accounts {
		accountID := strings.TrimSpace(account.ID)
		if accountID == "" || account.Disabled {
			continue
		}
		for _, apiKey := range account.APIKeys {
			key := strings.TrimSpace(apiKey.Key)
			if key == "" || apiKey.Disabled {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			records = append(records, keyRecord{
				Key:         key,
				AccountID:   accountID,
				AccountName: strings.TrimSpace(account.Name),
				APIKeyID:    defaultAPIKeyID(apiKey.ID, key),
				APIKeyName:  strings.TrimSpace(apiKey.Name),
				Metadata:    mergeMetadata(account.Metadata, apiKey.Metadata),
			})
		}
	}
	for _, key := range normalizeKeys(cfg.APIKeys) {
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		records = append(records, keyRecord{
			Key:         key,
			AccountID:   "default",
			AccountName: "Default",
			APIKeyID:    key,
			APIKeyName:  "Legacy API key",
		})
	}
	return records
}

func metadataFromKeyRecord(record keyRecord) map[string]string {
	metadata := make(map[string]string, len(record.Metadata)+4)
	for key, value := range record.Metadata {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		metadata[trimmedKey] = strings.TrimSpace(value)
	}
	setMetadata(metadata, sdkaccess.MetadataAccountID, record.AccountID)
	setMetadata(metadata, sdkaccess.MetadataAccountName, record.AccountName)
	setMetadata(metadata, sdkaccess.MetadataAPIKeyID, record.APIKeyID)
	setMetadata(metadata, sdkaccess.MetadataAPIKeyName, record.APIKeyName)
	return metadata
}

func mergeMetadata(accountMetadata, keyMetadata map[string]string) map[string]string {
	if len(accountMetadata)+len(keyMetadata) == 0 {
		return nil
	}
	merged := make(map[string]string, len(accountMetadata)+len(keyMetadata))
	for key, value := range accountMetadata {
		setMetadata(merged, key, value)
	}
	for key, value := range keyMetadata {
		setMetadata(merged, key, value)
	}
	return merged
}

func setMetadata(metadata map[string]string, key, value string) {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return
	}
	metadata[key] = value
}

func defaultAPIKeyID(id, key string) string {
	id = strings.TrimSpace(id)
	if id != "" {
		return id
	}
	return strings.TrimSpace(key)
}
