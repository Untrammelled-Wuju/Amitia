// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package config

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server            ServerConfig            `mapstructure:"server"`
	Storage           StorageConfig           `mapstructure:"storage"`
	JWT               JWTConfig               `mapstructure:"jwt"`
	Security          SecurityRuntimeConfig   `mapstructure:"security"`
	App               AppConfig               `mapstructure:"app"`
	Chat              ChatConfig              `mapstructure:"chat"`
	Qdrant            QdrantConfig            `mapstructure:"qdrant"`
	Embedding         EmbeddingConfig         `mapstructure:"embedding"`
	Surreal           SurrealConfig           `mapstructure:"surrealdb"`
	Prompt            PromptFeatureFlags      `mapstructure:"prompt"`
	DesktopPetRuntime DesktopPetRuntimeConfig `mapstructure:"desktopPetRuntime"`
	Runtime           RuntimeConfig           `mapstructure:"runtime"`
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Host string `mapstructure:"host"`
	Mode string `mapstructure:"mode"`
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

var AppCfg *Config

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

	bindRuntimeEnvironment(v)

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("配置解析失败: %w", err)
	}

	normalizeConfig(cfg)
	if err := validateRuntimeConfig(cfg); err != nil {
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
	v.SetDefault("qdrant.host", "127.0.0.1")
	v.SetDefault("qdrant.port", 19178)
	v.SetDefault("qdrant.collectionName", "memory_embeddings")
	v.SetDefault("qdrant.vectorDim", 2560)
	v.SetDefault("qdrant.limit", 10)
	v.SetDefault("qdrant.enabled", true)
	v.SetDefault("qdrant.collections.memory_embeddings.name", "memory_embeddings")
	v.SetDefault("qdrant.collections.memory_embeddings.vectorDim", 2560)
	v.SetDefault("qdrant.collections.working_memory.name", "working_memory")
	v.SetDefault("qdrant.collections.working_memory.vectorDim", 2560)
	v.SetDefault("qdrant.collections.user_profiles.name", "user_profiles")
	v.SetDefault("qdrant.collections.user_profiles.vectorDim", 2560)
	v.SetDefault("qdrant.collections.episodic_memories.name", "episodic_memories")
	v.SetDefault("qdrant.collections.episodic_memories.vectorDim", 2560)
	v.SetDefault("qdrant.collections.amitia_emotes.name", "amitia_emotes")
	v.SetDefault("qdrant.collections.amitia_emotes.vectorDim", 2560)
	v.SetDefault("embedding.modelName", "doubao-embedding-vision-251215")
	v.SetDefault("embedding.baseUrl", "")
	v.SetDefault("embedding.apiKey", "")
	v.SetDefault("surrealdb.host", "127.0.0.1")
	v.SetDefault("surrealdb.port", 18000)
	v.SetDefault("surrealdb.namespace", "uai")
	v.SetDefault("surrealdb.database", "memory_graph")
	v.SetDefault("surrealdb.username", "root")
	v.SetDefault("surrealdb.password", "root")
	v.SetDefault("surrealdb.dataPath", "data/graph.db")
	v.SetDefault("surrealdb.enabled", true)
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
}

func bindRuntimeEnvironment(v *viper.Viper) {
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
	{key: "qdrant.enabled", environments: []string{"AMITIA_QDRANT_ENABLED"}},
	{key: "qdrant.host", environments: []string{"AMITIA_QDRANT_HOST"}},
	{key: "qdrant.port", environments: []string{"AMITIA_QDRANT_PORT"}},
	{key: "qdrant.binaryPath", environments: []string{"AMITIA_QDRANT_BIN", "QDRANT_BIN"}},
	{key: "qdrant.dataDir", environments: []string{"AMITIA_QDRANT_DATA_DIR", "QDRANT_DATA_DIR"}},
	{key: "surrealdb.enabled", environments: []string{"AMITIA_SURREAL_ENABLED"}},
	{key: "surrealdb.host", environments: []string{"AMITIA_SURREAL_HOST"}},
	{key: "surrealdb.port", environments: []string{"AMITIA_SURREAL_PORT"}},
	{key: "surrealdb.binaryPath", environments: []string{"AMITIA_SURREAL_BIN", "SURREAL_BIN"}},
	{key: "surrealdb.dataPath", environments: []string{"AMITIA_SURREAL_DATA_PATH", "SURREAL_DATA_PATH"}},
	{key: "desktopPetRuntime.enabled", environments: []string{"AMITIA_DESKTOP_PET_RUNTIME_ENABLED"}},
}

func normalizeConfig(cfg *Config) {
	cfg.Runtime.Mode = strings.ToLower(strings.TrimSpace(cfg.Runtime.Mode))
	cfg.Runtime.Node.BinaryPath = strings.TrimSpace(cfg.Runtime.Node.BinaryPath)
	cfg.Runtime.Node.NPMPath = strings.TrimSpace(cfg.Runtime.Node.NPMPath)
	cfg.Runtime.Node.NPXPath = strings.TrimSpace(cfg.Runtime.Node.NPXPath)
	cfg.Runtime.Node.WorkDir = strings.TrimSpace(cfg.Runtime.Node.WorkDir)
	cfg.Runtime.PluginHost.EntryPath = strings.TrimSpace(cfg.Runtime.PluginHost.EntryPath)
	cfg.Runtime.PluginHost.WorkDir = strings.TrimSpace(cfg.Runtime.PluginHost.WorkDir)
	cfg.Runtime.TaskHost.EntryPath = strings.TrimSpace(cfg.Runtime.TaskHost.EntryPath)
	cfg.Runtime.TaskHost.WorkDir = strings.TrimSpace(cfg.Runtime.TaskHost.WorkDir)
	cfg.Runtime.Sidecars.Wechat.EntryPath = strings.TrimSpace(cfg.Runtime.Sidecars.Wechat.EntryPath)
	cfg.Runtime.Sidecars.Wechat.WorkDir = strings.TrimSpace(cfg.Runtime.Sidecars.Wechat.WorkDir)
	cfg.Runtime.Sidecars.Wechat.HealthURL = strings.TrimSpace(cfg.Runtime.Sidecars.Wechat.HealthURL)
	cfg.Runtime.Sidecars.QQ.EntryPath = strings.TrimSpace(cfg.Runtime.Sidecars.QQ.EntryPath)
	cfg.Runtime.Sidecars.QQ.WorkDir = strings.TrimSpace(cfg.Runtime.Sidecars.QQ.WorkDir)
	cfg.Runtime.Sidecars.QQ.HealthURL = strings.TrimSpace(cfg.Runtime.Sidecars.QQ.HealthURL)
	cfg.Qdrant.Host = strings.TrimSpace(cfg.Qdrant.Host)
	cfg.Qdrant.BinaryPath = strings.TrimSpace(cfg.Qdrant.BinaryPath)
	cfg.Qdrant.DataDir = strings.TrimSpace(cfg.Qdrant.DataDir)
	cfg.Surreal.Host = strings.TrimSpace(cfg.Surreal.Host)
	cfg.Surreal.BinaryPath = strings.TrimSpace(cfg.Surreal.BinaryPath)
	cfg.Surreal.DataPath = strings.TrimSpace(cfg.Surreal.DataPath)
	cfg.Server.Host = strings.TrimSpace(cfg.Server.Host)
	cfg.Security.Mode = strings.TrimSpace(cfg.Security.Mode)
	cfg.Security.LocalTokenFile = strings.TrimSpace(cfg.Security.LocalTokenFile)
	cfg.Security.LocalUserID = strings.TrimSpace(cfg.Security.LocalUserID)
	cfg.App.DeployMode = strings.TrimSpace(cfg.App.DeployMode)
}

func validateRuntimeConfig(cfg *Config) error {
	switch cfg.Runtime.Mode {
	case "", "desktop", "android-proot", "server":
		if cfg.Runtime.Mode == "" {
			cfg.Runtime.Mode = "desktop"
		}
	default:
		return fmt.Errorf("未知的 runtime.mode: %q，允许的值为: desktop, android-proot, server", cfg.Runtime.Mode)
	}

	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("无效的 server.port: %d，必须在 1-65535 范围内", cfg.Server.Port)
	}

	if cfg.Qdrant.Enabled && (cfg.Qdrant.Port < 1 || cfg.Qdrant.Port > 65535) {
		return fmt.Errorf("无效的 qdrant.port: %d，必须在 1-65535 范围内", cfg.Qdrant.Port)
	}
	if cfg.Surreal.Enabled && (cfg.Surreal.Port < 1 || cfg.Surreal.Port > 65535) {
		return fmt.Errorf("无效的 surrealdb.port: %d，必须在 1-65535 范围内", cfg.Surreal.Port)
	}
	if cfg.Runtime.Sidecars.Wechat.Enabled && (cfg.Runtime.Sidecars.Wechat.Port < 1 || cfg.Runtime.Sidecars.Wechat.Port > 65535) {
		return fmt.Errorf("无效的 wechat sidecar port: %d，必须在 1-65535 范围内", cfg.Runtime.Sidecars.Wechat.Port)
	}
	if cfg.Runtime.Sidecars.QQ.Enabled && (cfg.Runtime.Sidecars.QQ.Port < 1 || cfg.Runtime.Sidecars.QQ.Port > 65535) {
		return fmt.Errorf("无效的 qq sidecar port: %d，必须在 1-65535 范围内", cfg.Runtime.Sidecars.QQ.Port)
	}

	if cfg.Runtime.Sidecars.Wechat.Enabled && cfg.Runtime.Sidecars.Wechat.HealthURL != "" {
		u, err := url.Parse(cfg.Runtime.Sidecars.Wechat.HealthURL)
		if err != nil {
			return fmt.Errorf("无效的 wechat healthUrl: %w", err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("无效的 wechat healthUrl scheme: %s", u.Scheme)
		}
		if u.Host == "" {
			return fmt.Errorf("无效的 wechat healthUrl: 缺少 host")
		}
	}
	if cfg.Runtime.Sidecars.QQ.Enabled && cfg.Runtime.Sidecars.QQ.HealthURL != "" {
		u, err := url.Parse(cfg.Runtime.Sidecars.QQ.HealthURL)
		if err != nil {
			return fmt.Errorf("无效的 qq healthUrl: %w", err)
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return fmt.Errorf("无效的 qq healthUrl scheme: %s", u.Scheme)
		}
		if u.Host == "" {
			return fmt.Errorf("无效的 qq healthUrl: 缺少 host")
		}
	}

	if cfg.Runtime.Mode == "android-proot" {
		if !isLoopbackHost(cfg.Server.Host) {
			return fmt.Errorf("android-proot 模式下 server.host 必须是回环地址，当前: %s", cfg.Server.Host)
		}
		if cfg.Security.AllowRemoteAccess {
			return fmt.Errorf("android-proot 模式下 security.allowRemoteAccess 必须为 false")
		}
		if cfg.Qdrant.Enabled && !isLoopbackHost(cfg.Qdrant.Host) {
			return fmt.Errorf("android-proot 模式下 qdrant.host 必须是回环地址，当前: %s", cfg.Qdrant.Host)
		}
		if cfg.Surreal.Enabled && !isLoopbackHost(cfg.Surreal.Host) {
			return fmt.Errorf("android-proot 模式下 surrealdb.host 必须是回环地址，当前: %s", cfg.Surreal.Host)
		}
		if cfg.Runtime.Sidecars.Wechat.Enabled && cfg.Runtime.Sidecars.Wechat.HealthURL != "" {
			u, err := url.Parse(cfg.Runtime.Sidecars.Wechat.HealthURL)
			if err != nil {
				return fmt.Errorf("android-proot 模式下 wechat healthUrl 无效: %w", err)
			}
			if !isLoopbackHost(u.Hostname()) {
				return fmt.Errorf("android-proot 模式下 wechat healthUrl 必须指向回环地址，当前: %s", u.Hostname())
			}
		}
		if cfg.Runtime.Sidecars.QQ.Enabled && cfg.Runtime.Sidecars.QQ.HealthURL != "" {
			u, err := url.Parse(cfg.Runtime.Sidecars.QQ.HealthURL)
			if err != nil {
				return fmt.Errorf("android-proot 模式下 qq healthUrl 无效: %w", err)
			}
			if !isLoopbackHost(u.Hostname()) {
				return fmt.Errorf("android-proot 模式下 qq healthUrl 必须指向回环地址，当前: %s", u.Hostname())
			}
		}
	}

	return nil
}

func isLoopbackHost(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "127.0.0.1", "localhost", "::1":
		return true
	}
	return false
}

var weakSecretPatterns = []string{
	"",
	"u-ai-secret-key-change-me",
	"secret",
	"password",
	"123456",
	"change-me",
	"default",
}

func isStrongSecret(secret string) bool {
	if len(secret) < 16 {
		return false
	}
	for _, pattern := range weakSecretPatterns {
		if secret == pattern {
			return false
		}
	}
	return true
}

func (c *ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

type DesktopPetRuntimeConfig struct {
	Enabled               bool   `mapstructure:"enabled"`
	Path                  string `mapstructure:"path"`
	LoopbackOnly          bool   `mapstructure:"loopbackOnly"`
	AllowRemote           bool   `mapstructure:"allowRemote"`
	HeartbeatIntervalMs   int    `mapstructure:"heartbeatIntervalMs"`
	HeartbeatTimeoutMs    int    `mapstructure:"heartbeatTimeoutMs"`
	MaxMessageBytes       int    `mapstructure:"maxMessageBytes"`
	RegisterTimeoutSec    int    `mapstructure:"registerTimeoutSec"`
	SendQueueSize         int    `mapstructure:"sendQueueSize"`
	CommandTimeoutSec     int    `mapstructure:"commandTimeoutSec"`
	MaxRetryAttempts      int    `mapstructure:"maxRetryAttempts"`
	RetryBaseDelayMs      int    `mapstructure:"retryBaseDelayMs"`
	RetryMaxDelayMs       int    `mapstructure:"retryMaxDelayMs"`
	CommandRetentionHours int    `mapstructure:"commandRetentionHours"`
}

type SecurityRuntimeConfig struct {
	Mode              string   `mapstructure:"mode"`
	AllowRemoteAccess bool     `mapstructure:"allowRemoteAccess"`
	LocalToken        string   `mapstructure:"localToken"`
	LocalTokenFile    string   `mapstructure:"localTokenFile"`
	LocalUserID       string   `mapstructure:"localUserId"`
	AllowedOrigins    []string `mapstructure:"allowedOrigins"`
}
