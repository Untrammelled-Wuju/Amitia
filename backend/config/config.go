// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package config

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/viper"
	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/pkg/resourceuri"
)

type Config struct {
	Server            ServerConfig            `mapstructure:"server"`
	Storage           StorageConfig           `mapstructure:"storage"`
	Security          SecurityRuntimeConfig   `mapstructure:"security"`
	App               AppConfig               `mapstructure:"app"`
	Chat              ChatConfig              `mapstructure:"chat"`
	Embedding         EmbeddingConfig         `mapstructure:"embedding"`
	Prompt            PromptFeatureFlags      `mapstructure:"prompt"`
	DesktopPetRuntime DesktopPetRuntimeConfig `mapstructure:"desktopPetRuntime"`
	Runtime           RuntimeConfig           `mapstructure:"runtime"`
	Providers         ProvidersConfig         `mapstructure:"providers"`
	Components        ComponentsConfig        `mapstructure:"components"`
}

type ProvidersConfig struct {
	ScriptRuntime ScriptRuntimeProviderConfig  `mapstructure:"scriptRuntime"`
	VectorStore   VectorStoreProviderConfig    `mapstructure:"vectorStore"`
	GraphStore    GraphStoreProviderConfig     `mapstructure:"graphStore"`
	Browser       BrowserRuntimeProviderConfig `mapstructure:"browser"`
	Search        SearchRuntimeProviderConfig  `mapstructure:"search"`
}

type SearchProviderRouteRuntimeConfig struct {
	Preferred []string `mapstructure:"preferred"`
	Fallback  []string `mapstructure:"fallback"`
}

type SearchRuntimeProviderConfig struct {
	Enabled             bool                                        `mapstructure:"enabled"`
	DefaultProvider     string                                      `mapstructure:"defaultProvider"`
	DefaultLimit        int                                         `mapstructure:"defaultLimit"`
	MaxLimit            int                                         `mapstructure:"maxLimit"`
	TimeoutSec          int                                         `mapstructure:"timeoutSec"`
	MaxResponseBytes    int64                                       `mapstructure:"maxResponseBytes"`
	CacheTTLSec         int                                         `mapstructure:"cacheTtlSec"`
	CacheMaxEntries     int                                         `mapstructure:"cacheMaxEntries"`
	NegativeCacheTTLSec int                                         `mapstructure:"negativeCacheTtlSec"`
	CircuitFailures     int                                         `mapstructure:"circuitFailures"`
	CircuitOpenSec      int                                         `mapstructure:"circuitOpenSec"`
	Research            WebResearchRuntimeConfig                    `mapstructure:"research"`
	Providers           map[string]SearchProviderRuntimeConfig      `mapstructure:"providers"`
	Routes              map[string]SearchProviderRouteRuntimeConfig `mapstructure:"routes"`
}

type WebResearchRuntimeConfig struct {
	DeepResearchEnabled   bool    `mapstructure:"deepResearchEnabled"`
	BrowserEnabled        bool    `mapstructure:"browserEnabled"`
	PDFEnabled            bool    `mapstructure:"pdfEnabled"`
	PDFTextCommand        string  `mapstructure:"pdfTextCommand"`
	PDFInfoCommand        string  `mapstructure:"pdfInfoCommand"`
	PDFTimeoutSec         int     `mapstructure:"pdfTimeoutSec"`
	MaxExecutionSec       int     `mapstructure:"maxExecutionSec"`
	MaxToolOutputChars    int     `mapstructure:"maxToolOutputChars"`
	MaxQueries            int     `mapstructure:"maxQueries"`
	MaxProvidersPerQuery  int     `mapstructure:"maxProvidersPerQuery"`
	MaxSearchResults      int     `mapstructure:"maxSearchResults"`
	MaxResultsPerDomain   int     `mapstructure:"maxResultsPerDomain"`
	MaxOpenPages          int     `mapstructure:"maxOpenPages"`
	MaxDeepOpenPages      int     `mapstructure:"maxDeepOpenPages"`
	MaxDeepRounds         int     `mapstructure:"maxDeepRounds"`
	MaxDeepSearchCalls    int     `mapstructure:"maxDeepSearchCalls"`
	MaxSearchCalls        int     `mapstructure:"maxSearchCalls"`
	MaxProviderCostUSD    float64 `mapstructure:"maxProviderCostUsd"`
	MaxProviderCredits    float64 `mapstructure:"maxProviderCredits"`
	MinDeepNewSources     int     `mapstructure:"minDeepNewSources"`
	MaxParallelSearch     int     `mapstructure:"maxParallelSearch"`
	MaxParallelFetch      int     `mapstructure:"maxParallelFetch"`
	SearchTimeoutSec      int     `mapstructure:"searchTimeoutSec"`
	FetchTimeoutSec       int     `mapstructure:"fetchTimeoutSec"`
	MaxFetchBytes         int64   `mapstructure:"maxFetchBytes"`
	MaxPageChars          int     `mapstructure:"maxPageChars"`
	MaxEvidencePerPage    int     `mapstructure:"maxEvidencePerPage"`
	MaxEvidenceChars      int     `mapstructure:"maxEvidenceChars"`
	ReferenceTTLHours     int     `mapstructure:"referenceTtlHours"`
	PageCacheTTLSec       int     `mapstructure:"pageCacheTtlSec"`
	AutoBrowserEscalation bool    `mapstructure:"autoBrowserEscalation"`
	MinStaticContentChars int     `mapstructure:"minStaticContentChars"`
	MaxRedirects          int     `mapstructure:"maxRedirects"`
	MaxBrowserPages       int     `mapstructure:"maxBrowserPages"`
	MaxBrowserScrolls     int     `mapstructure:"maxBrowserScrolls"`
	MaxBrowserSeconds     int     `mapstructure:"maxBrowserSeconds"`
}

type SearchProviderRuntimeConfig struct {
	Type              string            `mapstructure:"type"`
	Endpoint          string            `mapstructure:"endpoint"`
	CredentialRef     string            `mapstructure:"credentialRef"`
	Enabled           bool              `mapstructure:"enabled"`
	Priority          int               `mapstructure:"priority"`
	Kinds             []string          `mapstructure:"kinds"`
	AllowHTTP         bool              `mapstructure:"allowHttp"`
	AllowPrivate      bool              `mapstructure:"allowPrivate"`
	EngineCredentials map[string]string `mapstructure:"engineCredentials"`
}

type BrowserRuntimeProviderConfig struct {
	Enabled                 bool     `mapstructure:"enabled"`
	ExecutablePath          string   `mapstructure:"executablePath"`
	Headless                bool     `mapstructure:"headless"`
	UserDataRoot            string   `mapstructure:"userDataRoot"`
	StartupTimeoutSec       int      `mapstructure:"startupTimeoutSec"`
	ShutdownTimeoutSec      int      `mapstructure:"shutdownTimeoutSec"`
	MaxBrowserMemoryBytes   int64    `mapstructure:"maxBrowserMemoryBytes"`
	AllowedSchemes          []string `mapstructure:"allowedSchemes"`
	MaxSessions             int      `mapstructure:"maxSessions"`
	MaxTabsPerSession       int      `mapstructure:"maxTabsPerSession"`
	MaxTabsTotal            int      `mapstructure:"maxTabsTotal"`
	NavigationTimeoutSec    int      `mapstructure:"navigationTimeoutSec"`
	MaxNavigationTimeoutSec int      `mapstructure:"maxNavigationTimeoutSec"`
}

type ScriptRuntimeProviderConfig struct {
	Enabled  bool              `mapstructure:"enabled"`
	Required bool              `mapstructure:"required"`
	Provider string            `mapstructure:"provider"`
	Node     NodeProcessConfig `mapstructure:"node"`
}

type NodeProcessConfig struct {
	BinaryPath string `mapstructure:"binaryPath"`
	NPMPath    string `mapstructure:"npmPath"`
	NPXPath    string `mapstructure:"npxPath"`
	WorkDir    string `mapstructure:"workDir"`
}

type VectorStoreProviderConfig struct {
	Enabled  bool         `mapstructure:"enabled"`
	Required bool         `mapstructure:"required"`
	Provider string       `mapstructure:"provider"`
	Qdrant   QdrantConfig `mapstructure:"qdrant"`
}

type GraphStoreProviderConfig struct {
	Enabled   bool          `mapstructure:"enabled"`
	Required  bool          `mapstructure:"required"`
	Provider  string        `mapstructure:"provider"`
	SurrealDB SurrealConfig `mapstructure:"surrealdb"`
}

type ComponentsConfig struct {
	TaskHost ProcessComponentConfig `mapstructure:"taskHost"`
}

type ProcessComponentConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	EntryURI string `mapstructure:"entryUri"`
	WorkURI  string `mapstructure:"workUri"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Host string `mapstructure:"host"`
	Mode string `mapstructure:"mode"`
}

func (c *ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

type StorageConfig struct {
	DataDir string `mapstructure:"dataDir"`
}

type AppConfig struct {
	Name       string `mapstructure:"name"`
	Version    string `mapstructure:"version"`
	DeployMode string `mapstructure:"deployMode"`
}

type ChatConfig struct {
	EventReplayRingSize     int `mapstructure:"eventReplayRingSize"`
	ContextWindowMaxRounds  int `mapstructure:"contextWindowMaxRounds"`
	AgentTurnTimeoutSeconds int `mapstructure:"agentTurnTimeoutSeconds"`
	AgentMaxParallelTools   int `mapstructure:"agentMaxParallelTools"`
}

type QdrantConfig struct {
	Host            string                      `mapstructure:"host"`
	Port            int                         `mapstructure:"port"`
	BinaryPath      string                      `mapstructure:"binaryPath"`
	ConfigDir       string                      `mapstructure:"configDir"`
	DataDir         string                      `mapstructure:"dataDir"`
	SnapshotsDir    string                      `mapstructure:"snapshotsDir"`
	CollectionName  string                      `mapstructure:"collectionName"`
	VectorDim       int                         `mapstructure:"vectorDim"`
	Limit           int                         `mapstructure:"limit"`
	Collections     map[string]CollectionConfig `mapstructure:"collections"`
	Enabled         bool                        `mapstructure:"enabled"`
	ResourceProfile string                      `mapstructure:"resourceProfile"`
}

type CollectionConfig struct {
	Name      string `mapstructure:"name"`
	VectorDim int    `mapstructure:"vectorDim"`
}

type EmbeddingConfig struct {
	ModelName string `mapstructure:"modelName"`
	BaseUrl   string `mapstructure:"baseUrl"`
	ApiKey    string `mapstructure:"apiKey"`
}

type SurrealConfig struct {
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	BinaryPath string `mapstructure:"binaryPath"`
	Namespace  string `mapstructure:"namespace"`
	Database   string `mapstructure:"database"`
	Username   string `mapstructure:"username"`
	Password   string `mapstructure:"password"`
	DataPath   string `mapstructure:"dataPath"`
	Enabled    bool   `mapstructure:"enabled"`
}

type PromptFeatureFlags struct {
	TextlibRawEnabled      bool `mapstructure:"textlibRawEnabled"`
	PersonalityRawEnabled  bool `mapstructure:"personalityRawEnabled"`
	EmotionFusionEnabled   bool `mapstructure:"emotionFusionEnabled"`
	IntimacyDefaultEnabled bool `mapstructure:"intimacyDefaultEnabled"`
	MemoryRawEnabled       bool `mapstructure:"memoryRawEnabled"`
	ReplySanitizerEnabled  bool `mapstructure:"replySanitizerEnabled"`
	ProactiveRawEnabled    bool `mapstructure:"proactiveRawEnabled"`
}

type RuntimeConfig struct {
	Mode       string                   `mapstructure:"mode"`
	Node       NodeRuntimeConfig        `mapstructure:"node"`
	TaskHost   ProcessHostRuntimeConfig `mapstructure:"taskHost"`
	IOSSandbox IOSSandboxRuntimeConfig  `mapstructure:"iosSandbox"`
}

type IOSSandboxRuntimeConfig struct {
	Enabled      bool              `mapstructure:"enabled"`
	WorkspaceURI string            `mapstructure:"workspaceUri"`
	RootfsURI    string            `mapstructure:"rootfsUri"`
	Environment  map[string]string `mapstructure:"environment"`
}

func (c IOSSandboxRuntimeConfig) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.RootfsURI == "" {
		return fmt.Errorf("ios sandbox: rootfsUri is required when enabled")
	}
	if _, err := resourceuri.Parse(c.RootfsURI); err != nil {
		return fmt.Errorf("ios sandbox: invalid rootfsUri: %w", err)
	}
	if c.WorkspaceURI != "" {
		if _, err := resourceuri.Parse(c.WorkspaceURI); err != nil {
			return fmt.Errorf("ios sandbox: invalid workspaceUri: %w", err)
		}
	}
	return nil
}

type NodeRuntimeConfig struct {
	BinaryPath string `mapstructure:"binaryPath"`
	NPMPath    string `mapstructure:"npmPath"`
	NPXPath    string `mapstructure:"npxPath"`
	WorkDir    string `mapstructure:"workDir"`
}

type ProcessHostRuntimeConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	EntryPath string `mapstructure:"entryPath"`
	WorkDir   string `mapstructure:"workDir"`
}

type DesktopPetRuntimeConfig struct {
	Enabled               bool `mapstructure:"enabled"`
	LoopbackOnly          bool `mapstructure:"loopbackOnly"`
	AllowRemote           bool `mapstructure:"allowRemote"`
	HeartbeatIntervalMs   int  `mapstructure:"heartbeatIntervalMs"`
	HeartbeatTimeoutMs    int  `mapstructure:"heartbeatTimeoutMs"`
	MaxMessageBytes       int  `mapstructure:"maxMessageBytes"`
	RegisterTimeoutSec    int  `mapstructure:"registerTimeoutSec"`
	SendQueueSize         int  `mapstructure:"sendQueueSize"`
	CommandTimeoutSec     int  `mapstructure:"commandTimeoutSec"`
	MaxRetryAttempts      int  `mapstructure:"maxRetryAttempts"`
	RetryBaseDelayMs      int  `mapstructure:"retryBaseDelayMs"`
	RetryMaxDelayMs       int  `mapstructure:"retryMaxDelayMs"`
	CommandRetentionHours int  `mapstructure:"commandRetentionHours"`
}

type SecurityRuntimeConfig struct {
	Mode              string   `mapstructure:"mode"`
	AllowRemoteAccess bool     `mapstructure:"allowRemoteAccess"`
	LocalToken        string   `mapstructure:"localToken"`
	LocalTokenFile    string   `mapstructure:"localTokenFile"`
	AllowedOrigins    []string `mapstructure:"allowedOrigins"`
	AuditHmacSecret   string   `mapstructure:"auditHmacSecret"`
	RecoveryPepper    string   `mapstructure:"recoveryPepper"`
}

func (c *Config) ScriptRuntimeConfig() *ScriptRuntimeProviderConfig {
	return &c.Providers.ScriptRuntime
}

func (c *Config) VectorStoreConfig() *VectorStoreProviderConfig {
	return &c.Providers.VectorStore
}

func (c *Config) GraphStoreConfig() *GraphStoreProviderConfig {
	return &c.Providers.GraphStore
}

func (c *Config) QdrantConfig() *QdrantConfig {
	return &c.Providers.VectorStore.Qdrant
}

func (c *Config) SurrealDBConfig() *SurrealConfig {
	return &c.Providers.GraphStore.SurrealDB
}

func (c *Config) BrowserRuntimeConfig() *BrowserRuntimeProviderConfig {
	return &c.Providers.Browser
}

func (c *Config) SearchRuntimeConfig() *SearchRuntimeProviderConfig {
	return &c.Providers.Search
}

var AppCfg *Config

var providerIDRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,127}$`)

func InitConfig(configPath string) {
	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}
	AppCfg = cfg
}

func loadConfig(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yml")
	v.AddConfigPath(configPath)
	v.AddConfigPath(".")

	setDefaults(v)

	if err := v.ReadInConfig(); err != nil {
		log.Printf("[Config] 未找到配置文件，使用默认值: %v", err)
	} else {
		log.Printf("[Config] 已加载配置: %s", v.ConfigFileUsed())
	}

	if err := bindEnvironment(v); err != nil {
		return nil, fmt.Errorf("环境变量绑定失败: %w", err)
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("配置解析失败: %w", err)
	}

	applyLegacyProviderConfig(v, cfg)
	normalizeConfig(cfg)

	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	v.WatchConfig()
	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.port", 18899)
	v.SetDefault("server.host", "127.0.0.1")
	v.SetDefault("server.mode", "debug")
	v.SetDefault("storage.dataDir", "../data")
	v.SetDefault("app.name", "U-Ai")
	v.SetDefault("app.version", "26.2.0-beta.1")
	v.SetDefault("app.deployMode", "desktop-local")
	v.SetDefault("chat.eventReplayRingSize", 4096)
	v.SetDefault("chat.contextWindowMaxRounds", 20)
	v.SetDefault("chat.agentTurnTimeoutSeconds", 1800)
	v.SetDefault("chat.agentMaxParallelTools", 4)
	v.SetDefault("embedding.modelName", "doubao-embedding-vision-251215")
	v.SetDefault("embedding.baseUrl", "")
	v.SetDefault("embedding.apiKey", "")
	v.SetDefault("prompt.textlibRawEnabled", true)
	v.SetDefault("prompt.personalityRawEnabled", true)
	v.SetDefault("prompt.emotionFusionEnabled", true)
	v.SetDefault("prompt.intimacyDefaultEnabled", true)
	v.SetDefault("prompt.memoryRawEnabled", true)
	v.SetDefault("prompt.replySanitizerEnabled", true)
	v.SetDefault("prompt.proactiveRawEnabled", true)
	v.SetDefault("security.mode", "local_single_user")
	v.SetDefault("security.allowRemoteAccess", false)
	v.SetDefault("security.localToken", "")
	v.SetDefault("security.localTokenFile", "security/local-token")
	v.SetDefault("security.allowedOrigins", []string{"app://amitia", "http://127.0.0.1", "http://localhost"})
	v.SetDefault("desktopPetRuntime.enabled", true)
	v.SetDefault("desktopPetRuntime.loopbackOnly", true)
	v.SetDefault("desktopPetRuntime.allowRemote", false)
	v.SetDefault("desktopPetRuntime.heartbeatIntervalMs", 10000)
	v.SetDefault("desktopPetRuntime.heartbeatTimeoutMs", 30000)
	v.SetDefault("desktopPetRuntime.maxMessageBytes", 1048576)
	v.SetDefault("desktopPetRuntime.registerTimeoutSec", 10)
	v.SetDefault("desktopPetRuntime.sendQueueSize", 64)
	v.SetDefault("desktopPetRuntime.commandTimeoutSec", 30)
	v.SetDefault("desktopPetRuntime.maxRetryAttempts", 5)
	v.SetDefault("desktopPetRuntime.retryBaseDelayMs", 500)
	v.SetDefault("desktopPetRuntime.retryMaxDelayMs", 30000)
	v.SetDefault("desktopPetRuntime.commandRetentionHours", 24)
	v.SetDefault("runtime.mode", "desktop")
	v.SetDefault("runtime.node.binaryPath", "")
	v.SetDefault("runtime.node.npmPath", "")
	v.SetDefault("runtime.node.npxPath", "")
	v.SetDefault("runtime.node.workDir", "")
	v.SetDefault("runtime.taskHost.enabled", true)
	v.SetDefault("runtime.taskHost.entryPath", "")
	v.SetDefault("runtime.taskHost.workDir", "")
	v.SetDefault("providers.scriptRuntime.enabled", true)
	v.SetDefault("providers.scriptRuntime.required", false)
	v.SetDefault("providers.scriptRuntime.provider", "builtin.node-process")
	v.SetDefault("providers.scriptRuntime.node.binaryPath", "")
	v.SetDefault("providers.scriptRuntime.node.npmPath", "")
	v.SetDefault("providers.scriptRuntime.node.npxPath", "")
	v.SetDefault("providers.scriptRuntime.node.workDir", "")
	v.SetDefault("providers.vectorStore.enabled", true)
	v.SetDefault("providers.vectorStore.required", false)
	v.SetDefault("providers.vectorStore.provider", "builtin.qdrant-process")
	v.SetDefault("providers.vectorStore.qdrant.host", "127.0.0.1")
	v.SetDefault("providers.vectorStore.qdrant.port", 19178)
	v.SetDefault("providers.vectorStore.qdrant.collectionName", "memory_embeddings")
	v.SetDefault("providers.vectorStore.qdrant.vectorDim", 2560)
	v.SetDefault("providers.vectorStore.qdrant.limit", 10)
	v.SetDefault("providers.vectorStore.qdrant.enabled", true)
	v.SetDefault("providers.vectorStore.qdrant.binaryPath", "")
	v.SetDefault("providers.vectorStore.qdrant.dataDir", "")
	v.SetDefault("providers.vectorStore.qdrant.resourceProfile", "auto")
	v.SetDefault("providers.vectorStore.qdrant.collections.memory_embeddings.name", "memory_embeddings")
	v.SetDefault("providers.vectorStore.qdrant.collections.memory_embeddings.vectorDim", 2560)
	v.SetDefault("providers.vectorStore.qdrant.collections.working_memory.name", "working_memory")
	v.SetDefault("providers.vectorStore.qdrant.collections.working_memory.vectorDim", 2560)
	v.SetDefault("providers.vectorStore.qdrant.collections.user_profiles.name", "user_profiles")
	v.SetDefault("providers.vectorStore.qdrant.collections.user_profiles.vectorDim", 2560)
	v.SetDefault("providers.vectorStore.qdrant.collections.episodic_memories.name", "episodic_memories")
	v.SetDefault("providers.vectorStore.qdrant.collections.episodic_memories.vectorDim", 2560)
	v.SetDefault("providers.vectorStore.qdrant.collections.amitia_emotes.name", "amitia_emotes")
	v.SetDefault("providers.vectorStore.qdrant.collections.amitia_emotes.vectorDim", 2560)
	v.SetDefault("providers.graphStore.enabled", true)
	v.SetDefault("providers.graphStore.required", false)
	v.SetDefault("providers.graphStore.provider", "builtin.surrealdb-process")
	v.SetDefault("providers.graphStore.surrealdb.host", "127.0.0.1")
	v.SetDefault("providers.graphStore.surrealdb.port", 18000)
	v.SetDefault("providers.graphStore.surrealdb.namespace", "uai")
	v.SetDefault("providers.graphStore.surrealdb.database", "memory_graph")
	v.SetDefault("providers.graphStore.surrealdb.username", "root")
	v.SetDefault("providers.graphStore.surrealdb.password", "")
	v.SetDefault("providers.graphStore.surrealdb.dataPath", "data/graph.db")
	v.SetDefault("providers.graphStore.surrealdb.enabled", true)
	v.SetDefault("providers.graphStore.surrealdb.binaryPath", "")
	v.SetDefault("providers.browser.enabled", false)
	v.SetDefault("providers.browser.executablePath", "")
	v.SetDefault("providers.browser.headless", true)
	v.SetDefault("providers.browser.userDataRoot", "")
	v.SetDefault("providers.browser.startupTimeoutSec", 30)
	v.SetDefault("providers.browser.shutdownTimeoutSec", 5)
	v.SetDefault("providers.browser.maxBrowserMemoryBytes", 0)
	v.SetDefault("providers.browser.allowedSchemes", []string{"http", "https"})
	v.SetDefault("providers.browser.maxSessions", 8)
	v.SetDefault("providers.browser.maxTabsPerSession", 8)
	v.SetDefault("providers.browser.maxTabsTotal", 32)
	v.SetDefault("providers.browser.navigationTimeoutSec", 30)
	v.SetDefault("providers.browser.maxNavigationTimeoutSec", 120)
	v.SetDefault("providers.search.enabled", false)
	v.SetDefault("providers.search.defaultProvider", "native")
	v.SetDefault("providers.search.defaultLimit", 8)
	v.SetDefault("providers.search.maxLimit", 20)
	v.SetDefault("providers.search.timeoutSec", 10)
	v.SetDefault("providers.search.maxResponseBytes", 2097152)
	v.SetDefault("providers.search.cacheTtlSec", 120)
	v.SetDefault("providers.search.cacheMaxEntries", 512)
	v.SetDefault("providers.search.negativeCacheTtlSec", 30)
	v.SetDefault("providers.search.circuitFailures", 3)
	v.SetDefault("providers.search.circuitOpenSec", 30)
	v.SetDefault("providers.search.research.deepResearchEnabled", true)
	v.SetDefault("providers.search.research.browserEnabled", true)
	v.SetDefault("providers.search.research.pdfEnabled", true)
	v.SetDefault("providers.search.research.pdfTextCommand", "pdftotext")
	v.SetDefault("providers.search.research.pdfInfoCommand", "pdfinfo")
	v.SetDefault("providers.search.research.pdfTimeoutSec", 20)
	v.SetDefault("providers.search.research.maxQueries", 4)
	v.SetDefault("providers.search.research.maxExecutionSec", 120)
	v.SetDefault("providers.search.research.maxToolOutputChars", 120000)
	v.SetDefault("providers.search.research.maxProvidersPerQuery", 2)
	v.SetDefault("providers.search.research.maxSearchResults", 12)
	v.SetDefault("providers.search.research.maxResultsPerDomain", 3)
	v.SetDefault("providers.search.research.maxOpenPages", 4)
	v.SetDefault("providers.search.research.maxDeepOpenPages", 6)
	v.SetDefault("providers.search.research.maxDeepRounds", 3)
	v.SetDefault("providers.search.research.maxDeepSearchCalls", 12)
	v.SetDefault("providers.search.research.maxSearchCalls", 12)
	v.SetDefault("providers.search.research.maxProviderCostUsd", 1.0)
	v.SetDefault("providers.search.research.maxProviderCredits", 20.0)
	v.SetDefault("providers.search.research.minDeepNewSources", 2)
	v.SetDefault("providers.search.research.maxParallelSearch", 4)
	v.SetDefault("providers.search.research.maxParallelFetch", 3)
	v.SetDefault("providers.search.research.searchTimeoutSec", 20)
	v.SetDefault("providers.search.research.fetchTimeoutSec", 20)
	v.SetDefault("providers.search.research.maxFetchBytes", 6291456)
	v.SetDefault("providers.search.research.maxPageChars", 60000)
	v.SetDefault("providers.search.research.maxEvidencePerPage", 12)
	v.SetDefault("providers.search.research.maxEvidenceChars", 3500)
	v.SetDefault("providers.search.research.referenceTtlHours", 168)
	v.SetDefault("providers.search.research.pageCacheTtlSec", 600)
	v.SetDefault("providers.search.research.autoBrowserEscalation", true)
	v.SetDefault("providers.search.research.minStaticContentChars", 240)
	v.SetDefault("providers.search.research.maxRedirects", 5)
	v.SetDefault("providers.search.research.maxBrowserPages", 2)
	v.SetDefault("providers.search.research.maxBrowserScrolls", 2)
	v.SetDefault("providers.search.research.maxBrowserSeconds", 30)
	v.SetDefault("components.taskHost.enabled", true)
	v.SetDefault("components.taskHost.entryUri", "")
	v.SetDefault("components.taskHost.workUri", "")
	v.SetDefault("runtime.iosSandbox.enabled", false)
	v.SetDefault("runtime.iosSandbox.workspaceUri", "")
	v.SetDefault("runtime.iosSandbox.rootfsUri", "")
}

func bindEnvironment(v *viper.Viper) error {
	r := strings.NewReplacer(".", "_")
	v.SetEnvKeyReplacer(r)

	for _, entry := range runtimeEnvEntries {
		for _, env := range entry.environments {
			val, ok := os.LookupEnv(env)
			if !ok || val == "" {
				continue
			}
			if err := v.BindEnv(entry.key, env); err != nil {
				return fmt.Errorf("bindEnv %s -> %s: %w", env, entry.key, err)
			}
		}
	}

	return nil
}

type runtimeEnvEntry struct {
	key          string
	environments []string
}

var runtimeEnvEntries = []runtimeEnvEntry{
	{key: "runtime.mode", environments: []string{"AMITIA_RUNTIME_MODE", "AMITIA_RUN_MODE"}},
	{key: "server.host", environments: []string{"AMITIA_SERVER_HOST"}},
	{key: "server.port", environments: []string{"AMITIA_SERVER_PORT"}},
	{key: "server.mode", environments: []string{"AMITIA_SERVER_MODE"}},
	{key: "storage.dataDir", environments: []string{"AMITIA_DATA_DIR"}},
	{key: "app.deployMode", environments: []string{"AMITIA_DEPLOY_MODE"}},
	{key: "security.mode", environments: []string{"AMITIA_SECURITY_MODE"}},
	{key: "security.allowRemoteAccess", environments: []string{"AMITIA_ALLOW_REMOTE_ACCESS"}},
	{key: "security.localToken", environments: []string{"AMITIA_LOCAL_TOKEN"}},
	{key: "security.localTokenFile", environments: []string{"AMITIA_LOCAL_TOKEN_FILE"}},
	{key: "runtime.node.binaryPath", environments: []string{"AMITIA_NODE_BIN"}},
	{key: "runtime.node.npmPath", environments: []string{"AMITIA_NPM_BIN"}},
	{key: "runtime.node.npxPath", environments: []string{"AMITIA_NPX_BIN"}},
	{key: "runtime.node.workDir", environments: []string{"AMITIA_NODE_WORK_DIR"}},
	{key: "runtime.taskHost.enabled", environments: []string{"AMITIA_TASK_HOST_ENABLED"}},
	{key: "runtime.taskHost.entryPath", environments: []string{"AMITIA_TASK_HOST_PATH"}},
	{key: "runtime.taskHost.workDir", environments: []string{"AMITIA_TASK_HOST_WORK_DIR"}},
	{key: "runtime.iosSandbox.enabled", environments: []string{"AMITIA_IOS_SANDBOX_ENABLED"}},
	{key: "providers.scriptRuntime.enabled", environments: []string{"AMITIA_SCRIPT_RUNTIME_ENABLED"}},
	{key: "providers.scriptRuntime.required", environments: []string{"AMITIA_SCRIPT_RUNTIME_REQUIRED"}},
	{key: "providers.scriptRuntime.provider", environments: []string{"AMITIA_SCRIPT_RUNTIME_PROVIDER"}},
	{key: "providers.scriptRuntime.node.binaryPath", environments: []string{"AMITIA_NODE_BIN"}},
	{key: "providers.scriptRuntime.node.npmPath", environments: []string{"AMITIA_NPM_BIN"}},
	{key: "providers.scriptRuntime.node.npxPath", environments: []string{"AMITIA_NPX_BIN"}},
	{key: "providers.scriptRuntime.node.workDir", environments: []string{"AMITIA_NODE_WORK_DIR"}},
	{key: "providers.vectorStore.enabled", environments: []string{"AMITIA_VECTOR_STORE_ENABLED"}},
	{key: "providers.vectorStore.required", environments: []string{"AMITIA_VECTOR_STORE_REQUIRED"}},
	{key: "providers.vectorStore.provider", environments: []string{"AMITIA_VECTOR_STORE_PROVIDER"}},
	{key: "providers.vectorStore.qdrant.host", environments: []string{"AMITIA_QDRANT_HOST"}},
	{key: "providers.vectorStore.qdrant.port", environments: []string{"AMITIA_QDRANT_PORT"}},
	{key: "providers.vectorStore.qdrant.binaryPath", environments: []string{"AMITIA_QDRANT_BINARY"}},
	{key: "providers.vectorStore.qdrant.dataDir", environments: []string{"AMITIA_QDRANT_DATA_DIR"}},
	{key: "providers.vectorStore.qdrant.collectionName", environments: []string{"AMITIA_QDRANT_COLLECTION"}},
	{key: "providers.vectorStore.qdrant.vectorDim", environments: []string{"AMITIA_QDRANT_VECTOR_DIM"}},
	{key: "providers.vectorStore.qdrant.limit", environments: []string{"AMITIA_QDRANT_LIMIT"}},
	{key: "providers.vectorStore.qdrant.enabled", environments: []string{"AMITIA_QDRANT_ENABLED"}},
	{key: "providers.vectorStore.qdrant.resourceProfile", environments: []string{"QDRANT_RESOURCE_PROFILE"}},
	{key: "providers.graphStore.enabled", environments: []string{"AMITIA_GRAPH_STORE_ENABLED"}},
	{key: "providers.graphStore.required", environments: []string{"AMITIA_GRAPH_STORE_REQUIRED"}},
	{key: "providers.graphStore.provider", environments: []string{"AMITIA_GRAPH_STORE_PROVIDER"}},
	{key: "providers.graphStore.surrealdb.host", environments: []string{"AMITIA_SURREAL_HOST"}},
	{key: "providers.graphStore.surrealdb.port", environments: []string{"AMITIA_SURREAL_PORT"}},
	{key: "providers.graphStore.surrealdb.binaryPath", environments: []string{"AMITIA_SURREAL_BINARY"}},
	{key: "providers.graphStore.surrealdb.namespace", environments: []string{"AMITIA_SURREAL_NAMESPACE"}},
	{key: "providers.graphStore.surrealdb.database", environments: []string{"AMITIA_SURREAL_DATABASE"}},
	{key: "providers.graphStore.surrealdb.username", environments: []string{"AMITIA_SURREAL_USER"}},
	{key: "providers.graphStore.surrealdb.password", environments: []string{"AMITIA_SURREAL_PASSWORD"}},
	{key: "providers.graphStore.surrealdb.dataPath", environments: []string{"AMITIA_SURREAL_DATA_PATH"}},
	{key: "providers.graphStore.surrealdb.enabled", environments: []string{"AMITIA_SURREAL_ENABLED"}},
	{key: "providers.search.enabled", environments: []string{"AMITIA_SEARCH_ENABLED"}},
	{key: "providers.search.defaultProvider", environments: []string{"AMITIA_SEARCH_DEFAULT_PROVIDER"}},
	{key: "components.taskHost.enabled", environments: []string{"AMITIA_TASK_HOST_ENABLED"}},
	{key: "components.taskHost.entryUri", environments: []string{"AMITIA_TASK_HOST_URI"}},
	{key: "components.taskHost.workUri", environments: []string{"AMITIA_TASK_HOST_WORK_URI"}},
	{key: "desktopPetRuntime.enabled", environments: []string{"AMITIA_DESKTOP_PET_RUNTIME_ENABLED"}},
}

func applyLegacyProviderConfig(v *viper.Viper, cfg *Config) {
	if v.InConfig("qdrant.host") && !v.InConfig("providers.vectorStore.qdrant.host") {
		log.Printf("[Config] 检测到遗留 qdrant 顶层配置，迁移到 providers.vectorStore.qdrant")
		cfg.Providers.VectorStore.Qdrant.Host = v.GetString("qdrant.host")
	}
	if v.InConfig("qdrant.port") && !v.InConfig("providers.vectorStore.qdrant.port") {
		cfg.Providers.VectorStore.Qdrant.Port = v.GetInt("qdrant.port")
	}
	if v.InConfig("qdrant.binaryPath") && !v.InConfig("providers.vectorStore.qdrant.binaryPath") {
		cfg.Providers.VectorStore.Qdrant.BinaryPath = v.GetString("qdrant.binaryPath")
	}
	if v.InConfig("qdrant.dataDir") && !v.InConfig("providers.vectorStore.qdrant.dataDir") {
		cfg.Providers.VectorStore.Qdrant.DataDir = v.GetString("qdrant.dataDir")
	}
	if v.InConfig("qdrant.collectionName") && !v.InConfig("providers.vectorStore.qdrant.collectionName") {
		cfg.Providers.VectorStore.Qdrant.CollectionName = v.GetString("qdrant.collectionName")
	}
	if v.InConfig("qdrant.vectorDim") && !v.InConfig("providers.vectorStore.qdrant.vectorDim") {
		cfg.Providers.VectorStore.Qdrant.VectorDim = v.GetInt("qdrant.vectorDim")
	}
	if v.InConfig("qdrant.limit") && !v.InConfig("providers.vectorStore.qdrant.limit") {
		cfg.Providers.VectorStore.Qdrant.Limit = v.GetInt("qdrant.limit")
	}
	if v.InConfig("qdrant.enabled") && !v.InConfig("providers.vectorStore.qdrant.enabled") {
		cfg.Providers.VectorStore.Qdrant.Enabled = v.GetBool("qdrant.enabled")
	}

	if v.InConfig("surrealdb.host") && !v.InConfig("providers.graphStore.surrealdb.host") {
		log.Printf("[Config] 检测到遗留 surrealdb 顶层配置，迁移到 providers.graphStore.surrealdb")
		cfg.Providers.GraphStore.SurrealDB.Host = v.GetString("surrealdb.host")
	}
	if v.InConfig("surrealdb.port") && !v.InConfig("providers.graphStore.surrealdb.port") {
		cfg.Providers.GraphStore.SurrealDB.Port = v.GetInt("surrealdb.port")
	}
	if v.InConfig("surrealdb.binaryPath") && !v.InConfig("providers.graphStore.surrealdb.binaryPath") {
		cfg.Providers.GraphStore.SurrealDB.BinaryPath = v.GetString("surrealdb.binaryPath")
	}
	if v.InConfig("surrealdb.namespace") && !v.InConfig("providers.graphStore.surrealdb.namespace") {
		cfg.Providers.GraphStore.SurrealDB.Namespace = v.GetString("surrealdb.namespace")
	}
	if v.InConfig("surrealdb.database") && !v.InConfig("providers.graphStore.surrealdb.database") {
		cfg.Providers.GraphStore.SurrealDB.Database = v.GetString("surrealdb.database")
	}
	if v.InConfig("surrealdb.username") && !v.InConfig("providers.graphStore.surrealdb.username") {
		cfg.Providers.GraphStore.SurrealDB.Username = v.GetString("surrealdb.username")
	}
	if v.InConfig("surrealdb.password") && !v.InConfig("providers.graphStore.surrealdb.password") {
		cfg.Providers.GraphStore.SurrealDB.Password = v.GetString("surrealdb.password")
	}
	if v.InConfig("surrealdb.dataPath") && !v.InConfig("providers.graphStore.surrealdb.dataPath") {
		cfg.Providers.GraphStore.SurrealDB.DataPath = v.GetString("surrealdb.dataPath")
	}
	if v.InConfig("surrealdb.enabled") && !v.InConfig("providers.graphStore.surrealdb.enabled") {
		cfg.Providers.GraphStore.SurrealDB.Enabled = v.GetBool("surrealdb.enabled")
	}
}

func normalizeConfig(cfg *Config) {
	if cfg.Providers.ScriptRuntime.Provider == "" {
		cfg.Providers.ScriptRuntime.Provider = "builtin.node-process"
	}
	if cfg.Providers.VectorStore.Provider == "" {
		cfg.Providers.VectorStore.Provider = "builtin.qdrant-process"
	}
	if cfg.Providers.GraphStore.Provider == "" {
		cfg.Providers.GraphStore.Provider = "builtin.surrealdb-process"
	}
}

func validateConfig(cfg *Config) error {
	if cfg.Providers.ScriptRuntime.Required && !cfg.Providers.ScriptRuntime.Enabled {
		return fmt.Errorf("scriptRuntime: required=true 但 enabled=false")
	}
	if cfg.Providers.VectorStore.Required && !cfg.Providers.VectorStore.Enabled {
		return fmt.Errorf("vectorStore: required=true 但 enabled=false")
	}
	if cfg.Providers.GraphStore.Required && !cfg.Providers.GraphStore.Enabled {
		return fmt.Errorf("graphStore: required=true 但 enabled=false")
	}

	if cfg.Providers.ScriptRuntime.Enabled {
		if err := validateProviderID(cfg.Providers.ScriptRuntime.Provider); err != nil {
			return fmt.Errorf("scriptRuntime.provider: %w", err)
		}
	}
	if cfg.Providers.VectorStore.Enabled {
		if err := validateProviderID(cfg.Providers.VectorStore.Provider); err != nil {
			return fmt.Errorf("vectorStore.provider: %w", err)
		}
	}
	if cfg.Providers.GraphStore.Enabled {
		if err := validateProviderID(cfg.Providers.GraphStore.Provider); err != nil {
			return fmt.Errorf("graphStore.provider: %w", err)
		}
	}

	if err := validateSearchRuntimeConfig(cfg.Providers.Search); err != nil {
		return fmt.Errorf("search: %w", err)
	}

	if err := validateComponentURI(cfg.Components.TaskHost.EntryURI); err != nil {
		return fmt.Errorf("taskHost.entryUri: %w", err)
	}
	if cfg.Providers.VectorStore.Qdrant.Enabled {
		if cfg.Providers.VectorStore.Qdrant.Port < 0 || cfg.Providers.VectorStore.Qdrant.Port > 65535 {
			return fmt.Errorf("qdrant.port 超有效范围: %d", cfg.Providers.VectorStore.Qdrant.Port)
		}
	}
	if cfg.Providers.GraphStore.SurrealDB.Enabled {
		if cfg.Providers.GraphStore.SurrealDB.Port < 0 || cfg.Providers.GraphStore.SurrealDB.Port > 65535 {
			return fmt.Errorf("surrealdb.port 超有效范围: %d", cfg.Providers.GraphStore.SurrealDB.Port)
		}
	}

	return nil
}

func validateSearchRuntimeConfig(cfg SearchRuntimeProviderConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if len(cfg.Providers) == 0 && !strings.EqualFold(strings.TrimSpace(cfg.DefaultProvider), search.ProviderNative) {
		return fmt.Errorf("enabled=true 但未配置 provider")
	}
	enabledCount := 0
	if len(cfg.Providers) == 0 && strings.EqualFold(strings.TrimSpace(cfg.DefaultProvider), search.ProviderNative) {
		enabledCount = 1
	}
	for id, provider := range cfg.Providers {
		id = strings.TrimSpace(id)
		if id == "" {
			return fmt.Errorf("provider ID 不能为空")
		}
		if err := validateProviderID(id); err != nil {
			return fmt.Errorf("provider %q: %w", id, err)
		}
		if !provider.Enabled {
			continue
		}
		enabledCount++
		providerType := strings.ToLower(strings.TrimSpace(provider.Type))
		switch providerType {
		case "brave", "native", "exa", "tavily":
		default:
			return fmt.Errorf("provider %q type 不受支持: %q", id, provider.Type)
		}
		if providerType == "native" {
			continue
		}
		endpoint := strings.TrimSpace(provider.Endpoint)
		if endpoint == "" {
			return fmt.Errorf("provider %q endpoint 不能为空", id)
		}
		parsedEndpoint, err := url.Parse(endpoint)
		if err != nil || parsedEndpoint.Host == "" || (parsedEndpoint.Scheme != "https" && parsedEndpoint.Scheme != "http") {
			return fmt.Errorf("provider %q endpoint 必须是有效 http/https URL", id)
		}
		if parsedEndpoint.Scheme == "http" && !provider.AllowHTTP {
			return fmt.Errorf("provider %q 使用 http endpoint 但 allowHttp=false", id)
		}
	}
	if enabledCount == 0 {
		return fmt.Errorf("enabled=true 但没有启用的 provider")
	}
	defaultID := strings.TrimSpace(cfg.DefaultProvider)
	if defaultID != "" {
		provider, ok := cfg.Providers[defaultID]
		if !strings.EqualFold(defaultID, search.ProviderNative) && (!ok || !provider.Enabled) {
			return fmt.Errorf("defaultProvider %q 未配置或未启用", defaultID)
		}
	}
	for routeName, route := range cfg.Routes {
		routeName = strings.ToLower(strings.TrimSpace(routeName))
		if routeName == "" {
			return fmt.Errorf("search route 名称不能为空")
		}
		seen := map[string]struct{}{}
		for _, providerID := range append(append([]string{}, route.Preferred...), route.Fallback...) {
			providerID = strings.TrimSpace(providerID)
			if providerID == "" {
				return fmt.Errorf("search route %q 包含空 provider ID", routeName)
			}
			if _, duplicate := seen[providerID]; duplicate {
				return fmt.Errorf("search route %q 重复引用 provider %q", routeName, providerID)
			}
			seen[providerID] = struct{}{}
			if strings.EqualFold(providerID, search.ProviderNative) {
				continue
			}
			if _, ok := cfg.Providers[providerID]; !ok {
				return fmt.Errorf("search route %q 引用未配置 provider %q", routeName, providerID)
			}
		}
	}
	if cfg.CacheTTLSec < -1 || cfg.CacheTTLSec > 24*60*60 {
		return fmt.Errorf("cacheTtlSec 必须为 -1（禁用）、0（默认）或不超过 86400 秒")
	}
	if cfg.NegativeCacheTTLSec < -1 || cfg.NegativeCacheTTLSec > 24*60*60 {
		return fmt.Errorf("negativeCacheTtlSec 必须为 -1（禁用）、0（默认）或不超过 86400 秒")
	}
	if cfg.CacheMaxEntries < 0 || cfg.CacheMaxEntries > 10000 {
		return fmt.Errorf("cacheMaxEntries 必须为 0（默认）或不超过 10000")
	}
	if cfg.CircuitFailures < 0 || cfg.CircuitFailures > 20 || cfg.CircuitOpenSec < 0 || cfg.CircuitOpenSec > 10*60 {
		return fmt.Errorf("provider circuit breaker 配置超出安全范围")
	}
	r := cfg.Research
	if r.PDFTimeoutSec < 0 || r.PDFTimeoutSec > 5*60 || r.MaxExecutionSec < 0 || r.MaxExecutionSec > 30*60 || r.MaxToolOutputChars < 0 || r.MaxToolOutputChars > 500000 ||
		r.MaxQueries < 0 || r.MaxQueries > 16 || r.MaxProvidersPerQuery < 0 || r.MaxProvidersPerQuery > 8 ||
		r.MaxSearchResults < 0 || r.MaxSearchResults > 100 || r.MaxResultsPerDomain < 0 || r.MaxResultsPerDomain > 20 || r.MaxOpenPages < 0 || r.MaxOpenPages > 32 ||
		r.MaxDeepOpenPages < 0 || r.MaxDeepOpenPages > 64 || r.MaxDeepRounds < 0 || r.MaxDeepRounds > 8 ||
		r.MaxDeepSearchCalls < 0 || r.MaxDeepSearchCalls > 64 || r.MaxSearchCalls < 0 || r.MaxSearchCalls > 64 ||
		r.MaxProviderCostUSD < 0 || r.MaxProviderCostUSD > 1000 || r.MaxProviderCredits < 0 || r.MaxProviderCredits > 100000 ||
		r.MaxParallelSearch < 0 || r.MaxParallelSearch > 16 ||
		r.MaxParallelFetch < 0 || r.MaxParallelFetch > 16 || r.MaxRedirects < 0 || r.MaxRedirects > 10 ||
		r.MaxBrowserPages < 0 || r.MaxBrowserPages > 16 || r.MaxBrowserScrolls < 0 || r.MaxBrowserScrolls > 16 ||
		r.MaxBrowserSeconds < 0 || r.MaxBrowserSeconds > 15*60 {
		return fmt.Errorf("research 预算配置超出安全范围")
	}
	if r.MaxFetchBytes < 0 || r.MaxFetchBytes > 32*1024*1024 || r.MaxPageChars < 0 || r.MaxPageChars > 250000 ||
		r.MaxEvidencePerPage < 0 || r.MaxEvidencePerPage > 64 || r.MaxEvidenceChars < 0 || r.MaxEvidenceChars > 16000 ||
		r.ReferenceTTLHours < 0 || r.ReferenceTTLHours > 24*90 || r.PageCacheTTLSec < -1 || r.PageCacheTTLSec > 24*60*60 {
		return fmt.Errorf("research 内容或 TTL 配置超出安全范围")
	}
	return nil
}

func validateProviderID(id string) error {
	if id == "" {
		return fmt.Errorf("provider ID 不能为空")
	}
	if len(id) < 3 {
		return fmt.Errorf("长度小于 3: %q", id)
	}
	if len(id) > 128 {
		return fmt.Errorf("长度大于 128: %q", id)
	}
	if !providerIDRegex.MatchString(id) {
		return fmt.Errorf("非法字符或格式: %q", id)
	}
	return nil
}

func validateHealthURL(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("无法解析: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("scheme 必须为 http 或 https: %q", u.Scheme)
	}
	if u.User != nil {
		return fmt.Errorf("不允许包含用户信息")
	}
	if u.Host == "" {
		return fmt.Errorf("必须包含 host")
	}
	return nil
}

func validateComponentURI(raw string) error {
	if raw == "" {
		return nil
	}
	_, err := resourceuri.Parse(raw)
	return err
}
