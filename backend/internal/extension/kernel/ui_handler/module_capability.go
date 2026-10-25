package ui_handler

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/ui_provider"
)

var providerModuleCapabilityKey = newProviderModuleCapabilityKey()

func newProviderModuleCapabilityKey() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err == nil {
		return key
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%d", time.Now().UnixNano(), os.Getpid())))
	return sum[:]
}

func providerModuleCapability(providerID string, expiresAt int64) string {
	mac := hmac.New(sha256.New, providerModuleCapabilityKey)
	_, _ = mac.Write([]byte(providerID))
	_, _ = mac.Write([]byte(":"))
	_, _ = mac.Write([]byte(strconv.FormatInt(expiresAt, 10)))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return strconv.FormatInt(expiresAt, 10) + "." + signature
}

func validProviderModuleCapability(providerID, token string) bool {
	expiryRaw, signatureRaw, ok := strings.Cut(strings.TrimSpace(token), ".")
	if !ok || expiryRaw == "" || signatureRaw == "" {
		return false
	}
	expiresAt, err := strconv.ParseInt(expiryRaw, 10, 64)
	if err != nil || time.Now().Unix() > expiresAt {
		return false
	}
	expected := providerModuleCapability(providerID, expiresAt)
	return hmac.Equal([]byte(expected), []byte(strings.TrimSpace(token)))
}

func signProviderModuleCapabilities(provider *ui_provider.ProviderDefinition) *ui_provider.ProviderDefinition {
	if provider == nil {
		return nil
	}
	clone := *provider
	clone.Entries = make(map[string]ui_provider.Entry, len(provider.Entries))
	expiresAt := time.Now().Add(time.Hour).Unix()
	token := providerModuleCapability(provider.ProviderID, expiresAt)
	for platform, entry := range provider.Entries {
		if entry.Type == ui_provider.EntryWebModule {
			entry.ModuleToken = token
		}
		clone.Entries[platform] = entry
	}
	return &clone
}

func signProviderModuleCapabilityList(providers []*ui_provider.ProviderDefinition) []*ui_provider.ProviderDefinition {
	signed := make([]*ui_provider.ProviderDefinition, 0, len(providers))
	for _, provider := range providers {
		signed = append(signed, signProviderModuleCapabilities(provider))
	}
	return signed
}

func signResolvedProviderModules(resolved map[ui_provider.Capability]*ui_provider.ProviderDefinition) map[ui_provider.Capability]*ui_provider.ProviderDefinition {
	signed := make(map[ui_provider.Capability]*ui_provider.ProviderDefinition, len(resolved))
	for capability, provider := range resolved {
		signed[capability] = signProviderModuleCapabilities(provider)
	}
	return signed
}
