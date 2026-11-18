package ui_handler

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/ui_contribution"
	"github.com/u-ai/backend/internal/extension/kernel/ui_provider"
	amitialog "log"
)

var providerModuleCapabilityKey = newProviderModuleCapabilityKey()

type runtimeModuleCapabilityData struct {
	ExtensionID    string `json:"extensionId"`
	PageID         string `json:"pageId"`
	PluginID       string `json:"pluginId"`
	EntryID        string `json:"entryId"`
	SpaceID        string `json:"spaceId"`
	DeviceID       string `json:"deviceId"`
	CharacterID    string `json:"characterId,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
	ExpiresAt      int64  `json:"expiresAt"`
}

func runtimeModuleCapability(data runtimeModuleCapabilityData) string {
	payload, err := json.Marshal(data)
	if err != nil {
		panic(err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, providerModuleCapabilityKey)
	_, _ = mac.Write([]byte("runtime-module:" + encoded))
	return encoded + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func validRuntimeModuleCapability(token string) (*runtimeModuleCapabilityData, bool) {
	encoded, signature, ok := strings.Cut(strings.TrimSpace(token), ".")
	if !ok || encoded == "" || signature == "" {
		return nil, false
	}
	mac := hmac.New(sha256.New, providerModuleCapabilityKey)
	_, _ = mac.Write([]byte("runtime-module:" + encoded))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return nil, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, false
	}
	var data runtimeModuleCapabilityData
	if err := json.Unmarshal(payload, &data); err != nil || data.ExpiresAt == 0 || time.Now().Unix() > data.ExpiresAt {
		return nil, false
	}
	return &data, true
}

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
	if _, err := mac.Write([]byte(providerID)); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	if _, err := mac.Write([]byte(":")); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	if _, err := mac.Write([]byte(strconv.FormatInt(expiresAt, 10))); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
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

func pageModuleCapability(extensionID, pageID string, expiresAt int64) string {
	mac := hmac.New(sha256.New, providerModuleCapabilityKey)
	if _, err := mac.Write([]byte("page:")); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	if _, err := mac.Write([]byte(extensionID)); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	if _, err := mac.Write([]byte(":")); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	if _, err := mac.Write([]byte(pageID)); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	if _, err := mac.Write([]byte(":")); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	if _, err := mac.Write([]byte(strconv.FormatInt(expiresAt, 10))); err != nil {
		amitialog.Printf("[original-error] %v", err)
	}
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return strconv.FormatInt(expiresAt, 10) + "." + signature
}

func validPageModuleCapability(extensionID, pageID, token string) bool {
	expiryRaw, signatureRaw, ok := strings.Cut(strings.TrimSpace(token), ".")
	if !ok || expiryRaw == "" || signatureRaw == "" {
		return false
	}
	expiresAt, err := strconv.ParseInt(expiryRaw, 10, 64)
	if err != nil || time.Now().Unix() > expiresAt {
		return false
	}
	expected := pageModuleCapability(extensionID, pageID, expiresAt)
	return hmac.Equal([]byte(expected), []byte(strings.TrimSpace(token)))
}

func contributionModuleCapability(extensionID, contributionID string, expiresAt int64) string {
	mac := hmac.New(sha256.New, providerModuleCapabilityKey)
	for _, value := range []string{"contribution:", extensionID, ":", contributionID, ":", strconv.FormatInt(expiresAt, 10)} {
		if _, err := mac.Write([]byte(value)); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
	}
	return strconv.FormatInt(expiresAt, 10) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func validContributionModuleCapability(extensionID, contributionID, token string) bool {
	expiryRaw, signatureRaw, ok := strings.Cut(strings.TrimSpace(token), ".")
	if !ok || expiryRaw == "" || signatureRaw == "" {
		return false
	}
	expiresAt, err := strconv.ParseInt(expiryRaw, 10, 64)
	if err != nil || time.Now().Unix() > expiresAt {
		return false
	}
	expected := contributionModuleCapability(extensionID, contributionID, expiresAt)
	return hmac.Equal([]byte(expected), []byte(strings.TrimSpace(token)))
}

func signUIContributionModuleCapabilities(def *ui_contribution.UIContributionDefinition) *ui_contribution.UIContributionDefinition {
	if def == nil {
		return nil
	}
	clone := *def
	clone.Entry = def.Entry
	if clone.Entry.Type == ui_contribution.SandboxClientModule || clone.Entry.Type == ui_contribution.SandboxMobileWebRuntime {
		clone.Entry.ModuleToken = contributionModuleCapability(
			string(def.ExtensionID),
			string(def.ContributionID),
			time.Now().Add(time.Hour).Unix(),
		)
	}
	return &clone
}

func signUIContributionModuleCapabilityList(defs []*ui_contribution.UIContributionDefinition) []*ui_contribution.UIContributionDefinition {
	signed := make([]*ui_contribution.UIContributionDefinition, 0, len(defs))
	for _, def := range defs {
		signed = append(signed, signUIContributionModuleCapabilities(def))
	}
	return signed
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
		if entry.Type == ui_provider.EntryWebModule || entry.Type == ui_provider.EntryMobileWebRuntime {
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
