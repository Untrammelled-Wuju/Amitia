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
	"github.com/u-ai/backend/pkg/resourceuri"
)

type Config struct {
	Server            ServerConfig            `mapstructure:"server"`
	Storage           StorageConfig           `mapstructure:"storage"`
	JWT               JWTConfig               `mapstructure:"jwt"`
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
	ScriptRuntime ScriptRuntimeProviderConfig `mapstructure:"scriptRuntime"`
	VectorStore   VectorStoreProviderConfig   `mapstructure:"vectorStore"`
	GraphStore    GraphStoreProviderConfig    `mapstructure:"graphStore"`
}

type ScriptRuntimeProviderConfig struct {
	Enabled  bool                `mapstructure:"enabled"`
	Required bool                `mapstructure:"required"`
	Provider string              `mapstructure:"provider"`
	Node     NodeProcessConfig   `mapstructure:"node"`
}

type NodeProcessConfig struct {
	BinaryPath string `mapstructure:"binaryPath"`
	NPMPath    string `mapstructure:"npmPath"`
	NPXPath    string `mapstructure:"npxPath"`
	WorkDir    string `mapstructure:"workDir"`
}

type VectorStoreProviderConfig struct {
	Enabled  bool            `mapstructure:"enabled"`
	Required bool            `mapstructure:"required"`
	Provider string          `mapstructure:"provider"`
	Qdrant   QdrantConfig    `mapstructure:"qdrant"`
}

type GraphStoreProviderConfig struct {
	Enabled  bool            `mapstructure:"enabled"`
	Required bool            `mapstructure:"required"`
	Provider string          `mapstructure:"provider"`
	SurrealDB SurrealConfig  `mapstructure:"surrealdb"`
}

type ComponentsConfig struct {
	PluginHost ProcessComponentConfig `mapstructure:"pluginHost"`
	TaskHost   ProcessComponentConfig `mapstructure:"taskHost"`
	Sidecars   SidecarsConfig         `mapstructure:"sidecars"`
}

type ProcessComponentConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	EntryURI  string `mapstructure:"entryUri"`
	WorkURI   string `mapstructure:"workUri"`
}

type SidecarsConfig struct {
	Wechat SidecarEntryConfig `mapstructure:"wechat"`
	QQ     SidecarEntryConfig `mapstructure:"qq"`
}

type SidecarEntryConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	EntryURI  string `mapstructure:"entryUri"`
	WorkURI   string `mapstructure:"workUri"`
	Port      int    `mapstructure:"port"`
	HealthURL string `mapstructure:"healthUrl"`
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

type JWTConfig struct {
	Secret     string `mapstructure:"secret"`
	Issuer     string `mapstructure:"issuer"`
	Audience   string `mapstructure:"audience"`
	ExpireDays int    `mapstructure:"expireDays"`
}

type AppConfig struct {
	Name       string `mapstructure:"name"`
	Version    string `mapstructure:"version"`
	DeployMode string `mapstructure:"deployMode"`
}

type ChatConfig struct {
	MergeWindowMs          int `mapstructure:"mergeWindowMs"`
	ContextWindowMaxRounds int `mapstructure:"contextWindowMaxRounds"`
}

type QdrantConfig struct {
	Host           string                      `mapstructure:"host"`
	Port           int                         `mapstructure:"port"`
	BinaryPath     string                      `mapstructure:"binaryPath"`
	DataDir        string                      `mapstructure:"dataDir"`
	CollectionName string                      `mapstructure:"collectionName"`
	VectorDim      int                         `mapstructure:"vectorDim"`
	Limit          int                         `mapstructure:"limit"`
	Collections    map[string]CollectionConfig `mapstructure:"collections"`
	Enabled        bool                        `mapstructure:"enabled"`
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
	PluginHost ProcessHostRuntimeConfig `mapstructure:"pluginHost"`
	TaskHost   ProcessHostRuntimeConfig `mapstructure:"taskHost"`
	Sidecars   SidecarRuntimeConfig     `mapstructure:"sidecars"`
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

type SidecarRuntimeConfig struct {
	Wechat ManagedSidecarConfig `mapstructure:"wechat"`
	QQ     ManagedSidecarConfig `mapstructure:"qq"`
}

type ManagedSidecarConfig struct {
	Enabled   bool   `mapstructure:"enabled"`
	EntryPath string `mapstructure:"entryPath"`
	WorkDir   string `mapstructure:"workDir"`
	Port      int    `mapstructure:"port"`
	HealthURL string `mapstructure:"healthUrl"`
}

type DesktopPetRuntimeConfig struct {
	Enabled              bool `mapstructure:"enabled"`
	LoopbackOnly         bool `mapstructure:"loopbackOnly"`
	AllowRemote          bool `mapstructure:"allowRemote"`
	HeartbeatIntervalMs  int  `mapstructure:"heartbeatIntervalMs"`
	HeartbeatTimeoutMs   int  `mapstructure:"heartbeatTimeoutMs"`
	MaxMessageBytes      int  `mapstructure:"maxMessageBytes"`
	RegisterTimeoutSec   int  `mapstructure:"registerTimeoutSec"`
	SendQueueSize        int  `mapstructure:"sendQueueSize"`
	CommandTimeoutSec    int  `mapstructure:"commandTimeoutSec"`
	MaxRetryAttempts     int  `mapstructure:"maxRetryAttempts"`
	RetryBaseDelayMs     int  `mapstructure:"retryBaseDelayMs"`
	RetryMaxDelayMs      int  `mapstructure:"retryMaxDelayMs"`
	CommandRetentionHours int `mapstructure:"commandRetentionHours"`
}

type SecurityRuntimeConfig struct {
	Mode              string   `mapstructure:"mode"`
	AllowRemoteAccess bool     `mapstructure:"allowRemoteAccess"`
	LocalToken        string   `mapstructure:"localToken"`
	LocalTokenFile    string   `mapstructure:"localTokenFile"`
	LocalUserID       string   `mapstructure:"localUserId"`
	AllowedOrigins    []string `mapstructure:"allowedOrigins"`
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

	if !isStrongSecret(cfg.JWT.Secret) {
		return nil, fmt.Errorf("JWT Secret 过弱或使用了默认模板值，请在 config.yml 中设置强密钥")
	}

	v.WatchConfig()
	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.port", 18899)
	v.SetDefault("server.host", "127.0.0.1")
	v.SetDefault("server.mode", "debug")
	v.SetDefault("storage.dataDir", "../data")
	v.SetDefault("jwt.secret", "")
	v.SetDefault("jwt.expireDays", 7)
	v.SetDefault("app.name", "U-Ai")
	v.SetDefault("app.version", "1.0.0-beta")
	v.SetDefault("app.deployMode", "desktop-local")
	v.SetDefault("chat.contextWindowMaxRounds", 20)
	v.SetDefault("chat.mergeWindowMs", 6000)
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
	v.SetDefault("security.localUserId", "1")
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
	v.SetDefault("runtime.pluginHost.enabled", true)
	v.SetDefault("runtime.pluginHost.entryPath", "")
	v.SetDefault("runtime.pluginHost.workDir", "")
	v.SetDefault("runtime.taskHost.enabled", true)
	v.SetDefault("runtime.taskHost.entryPath", "")
	v.SetDefault("runtime.taskHost.workDir", "")
	v.SetDefault("runtime.sidecars.wechat.enabled", true)
	v.SetDefault("runtime.sidecars.wechat.entryPath", "")
	v.SetDefault("runtime.sidecars.wechat.workDir", "")
	v.SetDefault("runtime.sidecars.wechat.port", 19876)
	v.SetDefault("runtime.sidecars.wechat.healthUrl", "http://127.0.0.1:19876/api/health")
	v.SetDefault("runtime.sidecars.qq.enabled", true)
	v.SetDefault("runtime.sidecars.qq.entryPath", "")
	v.SetDefault("runtime.sidecars.qq.workDir", "")
	v.SetDefault("runtime.sidecars.qq.port", 19877)
	v.SetDefault("runtime.sidecars.qq.healthUrl", "http://127.0.0.1:19877/api/health")
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
	v.SetDefault("providers.graphStore.surrealdb.password", "root")
	v.SetDefault("providers.graphStore.surrealdb.dataPath", "data/graph.db")
	v.SetDefault("providers.graphStore.surrealdb.enabled", true)
	v.SetDefault("providers.graphStore.surrealdb.binaryPath", "")
	v.SetDefault("components.pluginHost.enabled", true)
	v.SetDefault("components.pluginHost.entryUri", "")
	v.SetDefault("components.pluginHost.workUri", "")
	v.SetDefault("components.taskHost.enabled", true)
	v.SetDefault("components.taskHost.entryUri", "")
	v.SetDefault("components.taskHost.workUri", "")
	v.SetDefault("components.sidecars.wechat.enabled", true)
	v.SetDefault("components.sidecars.wechat.entryUri", "")
	v.SetDefault("components.sidecars.wechat.workUri", "")
	v.SetDefault("components.sidecars.wechat.port", 19876)
	v.SetDefault("components.sidecars.wechat.healthUrl", "http://127.0.0.1:19876/api/health")
	v.SetDefault("components.sidecars.qq.enabled", true)
	v.SetDefault("components.sidecars.qq.entryUri", "")
	v.SetDefault("components.sidecars.qq.workUri", "")
	v.SetDefault("components.sidecars.qq.port", 19877)
	v.SetDefault("components.sidecars.qq.healthUrl", "http://127.0.0.1:19877/api/health")
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
			_ = v.BindEnv(entry.key, env)
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
	{key: "security.localUserId", environments: []string{"AMITIA_LOCAL_USER_ID"}},
	{key: "runtime.node.binaryPath", environments: []string{"AMITIA_NODE_BIN"}},
	{key: "runtime.node.npmPath", environments: []string{"AMITIA_NPM_BIN"}},
	{key: "runtime.node.npxPath", environments: []string{"AMITIA_NPX_BIN"}},
	{key: "runtime.node.workDir", environments: []string{"AMITIA_NODE_WORK_DIR"}},
	{key: "runtime.pluginHost.enabled", environments: []string{"AMITIA_PLUGIN_HOST_ENABLED"}},
	{key: "runtime.pluginHost.entryPath", environments: []string{"AMITIA_PLUGIN_HOST_PATH"}},
	{key: "runtime.pluginHost.workDir", environments: []string{"AMITIA_PLUGIN_HOST_WORK_DIR"}},
	{key: "runtime.taskHost.enabled", environments: []string{"AMITIA_TASK_HOST_ENABLED"}},
	{key: "runtime.taskHost.entryPath", environments: []string{"AMITIA_TASK_HOST_PATH"}},
	{key: "runtime.taskHost.workDir", environments: []string{"AMITIA_TASK_HOST_WORK_DIR"}},
	{key: "runtime.sidecars.wechat.enabled", environments: []string{"AMITIA_WECHAT_SIDECAR_ENABLED"}},
	{key: "runtime.sidecars.wechat.entryPath", environments: []string{"AMITIA_WECHAT_SIDECAR_PATH"}},
	{key: "runtime.sidecars.wechat.workDir", environments: []string{"AMITIA_WECHAT_SIDECAR_WORK_DIR"}},
	{key: "runtime.sidecars.wechat.port", environments: []string{"AMITIA_WECHAT_SIDECAR_PORT"}},
	{key: "runtime.sidecars.wechat.healthUrl", environments: []string{"AMITIA_WECHAT_SIDECAR_HEALTH_URL"}},
	{key: "runtime.sidecars.qq.enabled", environments: []string{"AMITIA_QQ_SIDECAR_ENABLED"}},
	{key: "runtime.sidecars.qq.entryPath", environments: []string{"AMITIA_QQ_SIDECAR_PATH"}},
	{key: "runtime.sidecars.qq.workDir", environments: []string{"AMITIA_QQ_SIDECAR_WORK_DIR"}},
	{key: "runtime.sidecars.qq.port", environments: []string{"AMITIA_QQ_SIDECAR_PORT"}},
	{key: "runtime.sidecars.qq.healthUrl", environments: []string{"AMITIA_QQ_SIDECAR_HEALTH_URL"}},
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
	{key: "components.pluginHost.enabled", environments: []string{"AMITIA_PLUGIN_HOST_ENABLED"}},
	{key: "components.pluginHost.entryUri", environments: []string{"AMITIA_PLUGIN_HOST_URI"}},
	{key: "components.pluginHost.workUri", environments: []string{"AMITIA_PLUGIN_HOST_WORK_URI"}},
	{key: "components.taskHost.enabled", environments: []string{"AMITIA_TASK_HOST_ENABLED"}},
	{key: "components.taskHost.entryUri", environments: []string{"AMITIA_TASK_HOST_URI"}},
	{key: "components.taskHost.workUri", environments: []string{"AMITIA_TASK_HOST_WORK_URI"}},
	{key: "components.sidecars.wechat.enabled", environments: []string{"AMITIA_WECHAT_SIDECAR_ENABLED"}},
	{key: "components.sidecars.wechat.entryUri", environments: []string{"AMITIA_WECHAT_SIDECAR_URI"}},
	{key: "components.sidecars.wechat.workUri", environments: []string{"AMITIA_WECHAT_SIDECAR_WORK_URI"}},
	{key: "components.sidecars.wechat.port", environments: []string{"AMITIA_WECHAT_SIDECAR_PORT"}},
	{key: "components.sidecars.wechat.healthUrl", environments: []string{"AMITIA_WECHAT_SIDECAR_HEALTH_URL"}},
	{key: "components.sidecars.qq.enabled", environments: []string{"AMITIA_QQ_SIDECAR_ENABLED"}},
	{key: "components.sidecars.qq.entryUri", environments: []string{"AMITIA_QQ_SIDECAR_URI"}},
	{key: "components.sidecars.qq.workUri", environments: []string{"AMITIA_QQ_SIDECAR_WORK_URI"}},
	{key: "components.sidecars.qq.port", environments: []string{"AMITIA_QQ_SIDECAR_PORT"}},
	{key: "components.sidecars.qq.healthUrl", environments: []string{"AMITIA_QQ_SIDECAR_HEALTH_URL"}},
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

	if err := validateComponentURI(cfg.Components.PluginHost.EntryURI); err != nil {
		return fmt.Errorf("pluginHost.entryUri: %w", err)
	}
	if err := validateComponentURI(cfg.Components.TaskHost.EntryURI); err != nil {
		return fmt.Errorf("taskHost.entryUri: %w", err)
	}
	if err := validateComponentURI(cfg.Components.Sidecars.Wechat.EntryURI); err != nil {
		return fmt.Errorf("sidecars.wechat.entryUri: %w", err)
	}
	if err := validateComponentURI(cfg.Components.Sidecars.QQ.EntryURI); err != nil {
		return fmt.Errorf("sidecars.qq.entryUri: %w", err)
	}

	if err := validateHealthURL(cfg.Components.Sidecars.Wechat.HealthURL); err != nil {
		return fmt.Errorf("sidecars.wechat.healthUrl: %w", err)
	}
	if err := validateHealthURL(cfg.Components.Sidecars.QQ.HealthURL); err != nil {
		return fmt.Errorf("sidecars.qq.healthUrl: %w", err)
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

func isStrongSecret(secret string) bool {
	return len(secret) >= 16 && !strings.Contains(secret, "change") && !strings.Contains(secret, "default")
}
