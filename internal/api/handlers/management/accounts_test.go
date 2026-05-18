package management

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

func TestPatchAccountUpsertsByID(t *testing.T) {
	h := newAccountTestHandler(t, &config.Config{})
	w := performAccountRequest(h.PatchAccount, http.MethodPatch, "/accounts", `{"id":"team-a","name":"Team A","api-keys":[{"id":"ci","key":"acct-key"}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if len(h.cfg.Accounts) != 1 || h.cfg.Accounts[0].ID != "team-a" || h.cfg.Accounts[0].APIKeys[0].Key != "acct-key" {
		t.Fatalf("accounts = %#v", h.cfg.Accounts)
	}
}

func TestPatchAccountAPIKeyUpsertsByID(t *testing.T) {
	h := newAccountTestHandler(t, &config.Config{SDKConfig: config.SDKConfig{Accounts: []config.ClientAccount{{ID: "team-a"}}}})
	w := performAccountRequest(func(c *gin.Context) {
		c.Params = gin.Params{{Key: "id", Value: "team-a"}}
		h.PatchAccountAPIKey(c)
	}, http.MethodPatch, "/accounts/team-a/api-keys", `{"id":"ci","name":"CI","key":"acct-key"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	keys := h.cfg.Accounts[0].APIKeys
	if len(keys) != 1 || keys[0].ID != "ci" || keys[0].Name != "CI" || keys[0].Key != "acct-key" {
		t.Fatalf("keys = %#v", keys)
	}
}

func TestDeleteAccountAPIKeyDeletesByID(t *testing.T) {
	h := newAccountTestHandler(t, &config.Config{SDKConfig: config.SDKConfig{Accounts: []config.ClientAccount{{ID: "team-a", APIKeys: []config.ClientAPIKey{{ID: "ci", Key: "acct-key"}}}}}})
	w := performAccountRequest(func(c *gin.Context) {
		c.Params = gin.Params{{Key: "id", Value: "team-a"}, {Key: "key_id", Value: "ci"}}
		h.DeleteAccountAPIKey(c)
	}, http.MethodDelete, "/accounts/team-a/api-keys/ci", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if len(h.cfg.Accounts[0].APIKeys) != 0 {
		t.Fatalf("keys = %#v", h.cfg.Accounts[0].APIKeys)
	}
}

func TestPutAccountsReplacesList(t *testing.T) {
	h := newAccountTestHandler(t, &config.Config{})
	w := performAccountRequest(h.PutAccounts, http.MethodPut, "/accounts", `[{"id":"team-a"},{"id":"team-b"}]`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if len(h.cfg.Accounts) != 2 || h.cfg.Accounts[1].ID != "team-b" {
		t.Fatalf("accounts = %#v", h.cfg.Accounts)
	}
}

func TestGetAccountsReturnsAccounts(t *testing.T) {
	h := newAccountTestHandler(t, &config.Config{SDKConfig: config.SDKConfig{Accounts: []config.ClientAccount{{ID: "team-a"}}}})
	w := performAccountRequest(h.GetAccounts, http.MethodGet, "/accounts", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	body := decodeJSONBody[struct {
		Accounts []config.ClientAccount `json:"accounts"`
	}](t, w)
	if len(body.Accounts) != 1 || body.Accounts[0].ID != "team-a" {
		t.Fatalf("body = %#v", body)
	}
}

func newAccountTestHandler(t *testing.T, cfg *config.Config) *Handler {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := config.SaveConfigPreserveComments(path, cfg); err != nil {
		t.Fatalf("SaveConfigPreserveComments() error = %v", err)
	}
	return &Handler{cfg: cfg, configFilePath: path}
}

func performAccountRequest(handler gin.HandlerFunc, method, path, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	handler(c)
	return w
}

func decodeJSONBody[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("json.Unmarshal() error = %v body=%s", err, w.Body.String())
	}
	return out
}
