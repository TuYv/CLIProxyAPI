package management

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v7/internal/config"
)

var (
	errAccountIDRequired      = errors.New("account id is required")
	errDuplicateAccountID     = errors.New("duplicate account id")
	errAPIKeyIDAndKeyRequired = errors.New("api key id and key are required")
	errDuplicateAPIKeyID      = errors.New("duplicate api key id")
)

func (h *Handler) GetAccounts(c *gin.Context) {
	h.mu.Lock()
	accounts := append([]config.ClientAccount(nil), h.cfg.Accounts...)
	h.mu.Unlock()
	c.JSON(http.StatusOK, gin.H{"accounts": accounts})
}

func (h *Handler) PutAccounts(c *gin.Context) {
	var accounts []config.ClientAccount
	if err := c.ShouldBindJSON(&accounts); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid accounts body"})
		return
	}
	if err := validateAccounts(accounts); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.mu.Lock()
	h.cfg.Accounts = accounts
	h.persistLocked(c)
	h.mu.Unlock()
}

func (h *Handler) PatchAccount(c *gin.Context) {
	var account config.ClientAccount
	if err := c.ShouldBindJSON(&account); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid account body"})
		return
	}
	account.ID = strings.TrimSpace(account.ID)
	if account.ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errAccountIDRequired.Error()})
		return
	}
	if err := validateAccount(account); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	h.mu.Lock()
	upsertAccount(&h.cfg.Accounts, account)
	h.persistLocked(c)
	h.mu.Unlock()
}

func (h *Handler) DeleteAccount(c *gin.Context) {
	id := strings.TrimSpace(c.Query("id"))
	if id == "" {
		id = strings.TrimSpace(c.Param("id"))
	}
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errAccountIDRequired.Error()})
		return
	}
	h.mu.Lock()
	accounts := h.cfg.Accounts[:0]
	for _, account := range h.cfg.Accounts {
		if strings.TrimSpace(account.ID) != id {
			accounts = append(accounts, account)
		}
	}
	h.cfg.Accounts = accounts
	h.persistLocked(c)
	h.mu.Unlock()
}

func (h *Handler) PatchAccountAPIKey(c *gin.Context) {
	accountID := strings.TrimSpace(c.Param("id"))
	if accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errAccountIDRequired.Error()})
		return
	}
	var apiKey config.ClientAPIKey
	if err := c.ShouldBindJSON(&apiKey); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid api key body"})
		return
	}
	if strings.TrimSpace(apiKey.ID) == "" || strings.TrimSpace(apiKey.Key) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": errAPIKeyIDAndKeyRequired.Error()})
		return
	}
	h.mu.Lock()
	account := findAccount(h.cfg.Accounts, accountID)
	if account == nil {
		h.mu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}
	upsertAccountAPIKey(&account.APIKeys, apiKey)
	h.persistLocked(c)
	h.mu.Unlock()
}

func (h *Handler) DeleteAccountAPIKey(c *gin.Context) {
	accountID := strings.TrimSpace(c.Param("id"))
	keyID := strings.TrimSpace(c.Param("key_id"))
	if accountID == "" || keyID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "account id and api key id are required"})
		return
	}
	h.mu.Lock()
	account := findAccount(h.cfg.Accounts, accountID)
	if account == nil {
		h.mu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}
	keys := account.APIKeys[:0]
	for _, apiKey := range account.APIKeys {
		if strings.TrimSpace(apiKey.ID) != keyID {
			keys = append(keys, apiKey)
		}
	}
	account.APIKeys = keys
	h.persistLocked(c)
	h.mu.Unlock()
}

func validateAccounts(accounts []config.ClientAccount) error {
	seen := make(map[string]struct{}, len(accounts))
	for _, account := range accounts {
		if err := validateAccount(account); err != nil {
			return err
		}
		id := strings.TrimSpace(account.ID)
		if _, exists := seen[id]; exists {
			return errDuplicateAccountID
		}
		seen[id] = struct{}{}
	}
	return nil
}

func validateAccount(account config.ClientAccount) error {
	if strings.TrimSpace(account.ID) == "" {
		return errAccountIDRequired
	}
	seen := make(map[string]struct{}, len(account.APIKeys))
	for _, apiKey := range account.APIKeys {
		id := strings.TrimSpace(apiKey.ID)
		if id == "" || strings.TrimSpace(apiKey.Key) == "" {
			return errAPIKeyIDAndKeyRequired
		}
		if _, exists := seen[id]; exists {
			return errDuplicateAPIKeyID
		}
		seen[id] = struct{}{}
	}
	return nil
}

func findAccount(accounts []config.ClientAccount, id string) *config.ClientAccount {
	for index := range accounts {
		if strings.TrimSpace(accounts[index].ID) == id {
			return &accounts[index]
		}
	}
	return nil
}

func upsertAccount(accounts *[]config.ClientAccount, account config.ClientAccount) {
	for index := range *accounts {
		if strings.TrimSpace((*accounts)[index].ID) == account.ID {
			(*accounts)[index] = account
			return
		}
	}
	*accounts = append(*accounts, account)
}

func upsertAccountAPIKey(keys *[]config.ClientAPIKey, apiKey config.ClientAPIKey) {
	apiKey.ID = strings.TrimSpace(apiKey.ID)
	apiKey.Key = strings.TrimSpace(apiKey.Key)
	for index := range *keys {
		if strings.TrimSpace((*keys)[index].ID) == apiKey.ID {
			(*keys)[index] = apiKey
			return
		}
	}
	*keys = append(*keys, apiKey)
}
