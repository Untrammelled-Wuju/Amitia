// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package adminrelease

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr                         string
	DataDir                      string
	MySQL                        MySQLConfig
	PublishRoot                  string
	DesktopPublishDir            string
	AndroidPublishDir            string
	PublicBaseURL                string
	CookieName                   string
	CookieSecure                 bool
	SessionTTL                   time.Duration
	AllowedOrigins               []string
	MaxUploadBytes               int64
	AndroidPackageName           string
	BootstrapUsername            string
	BootstrapPassword            string
	AndroidManifestKeyPath       string
	AndroidManifestKeyPassphrase string
	AdminWebDir                  string
}

type MySQLConfig struct {
	Host            string
	Port            int
	Username        string
	Password        string
	Database        string
	Charset         string
	Loc             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

func LoadConfigFromEnv() Config {
	root := detectProjectRoot()
	dataDir := envOrDefault("AMITIA_ADMIN_DATA_DIR", filepath.Join(root, "admin-data"))
	publishRoot := envOrDefault("AMITIA_ADMIN_PUBLISH_ROOT", filepath.Join(dataDir, "publish"))
	desktopDir := envOrDefault("AMITIA_ADMIN_DESKTOP_PUBLISH_DIR", filepath.Join(publishRoot, "amitia"))
	androidDir := envOrDefault("AMITIA_ADMIN_ANDROID_PUBLISH_DIR", filepath.Join(publishRoot, "amitia", "android"))
	cfg := Config{
		Addr:    envOrDefault("AMITIA_ADMIN_ADDR", "127.0.0.1:18998"),
		DataDir: dataDir,
		MySQL: MySQLConfig{
			Host:            envOrDefault("AMITIA_ADMIN_MYSQL_HOST", "127.0.0.1"),
			Port:            envInt("AMITIA_ADMIN_MYSQL_PORT", 3306),
			Username:        envOrDefault("AMITIA_ADMIN_MYSQL_USER", "amitia_admin"),
			Password:        os.Getenv("AMITIA_ADMIN_MYSQL_PASSWORD"),
			Database:        envOrDefault("AMITIA_ADMIN_MYSQL_DATABASE", "amitia_admin"),
			Charset:         envOrDefault("AMITIA_ADMIN_MYSQL_CHARSET", "utf8mb4"),
			Loc:             envOrDefault("AMITIA_ADMIN_MYSQL_LOC", "Local"),
			MaxOpenConns:    envInt("AMITIA_ADMIN_MYSQL_MAX_OPEN_CONNS", 50),
			MaxIdleConns:    envInt("AMITIA_ADMIN_MYSQL_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: time.Duration(envInt("AMITIA_ADMIN_MYSQL_CONN_MAX_LIFETIME_MINUTES", 60)) * time.Minute,
		},
		PublishRoot:                  publishRoot,
		DesktopPublishDir:            desktopDir,
		AndroidPublishDir:            androidDir,
		PublicBaseURL:                strings.TrimRight(envOrDefault("AMITIA_ADMIN_PUBLIC_BASE_URL", "https://amitia.untrammelled.top/amitia"), "/"),
		CookieName:                   envOrDefault("AMITIA_ADMIN_COOKIE_NAME", "amitia_admin_session"),
		CookieSecure:                 envBool("AMITIA_ADMIN_COOKIE_SECURE", false),
		SessionTTL:                   time.Duration(envInt("AMITIA_ADMIN_SESSION_HOURS", 12)) * time.Hour,
		AllowedOrigins:               envList("AMITIA_ADMIN_ALLOWED_ORIGINS", []string{"http://127.0.0.1:15179", "http://localhost:15179"}),
		MaxUploadBytes:               int64(envInt64("AMITIA_ADMIN_MAX_UPLOAD_BYTES", 4*1024*1024*1024)),
		AndroidPackageName:           envOrDefault("AMITIA_ADMIN_ANDROID_PACKAGE_NAME", "com.amitia.amitia_app"),
		BootstrapUsername:            envOrDefault("AMITIA_ADMIN_BOOTSTRAP_USERNAME", "untrammelled"),
		BootstrapPassword:            envOrDefault("AMITIA_ADMIN_BOOTSTRAP_PASSWORD", "925011Mc"),
		AndroidManifestKeyPath:       os.Getenv("AMITIA_ADMIN_ANDROID_MANIFEST_KEY_PATH"),
		AndroidManifestKeyPassphrase: os.Getenv("AMITIA_ADMIN_ANDROID_MANIFEST_KEY_PASSPHRASE"),
		AdminWebDir:                  envOrDefault("AMITIA_ADMIN_WEB_DIR", filepath.Join(root, "admin-system", "dist")),
	}
	return cfg
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Addr) == "" {
		return errors.New("AMITIA_ADMIN_ADDR 不能为空")
	}
	if c.MySQL.Host == "" {
		return errors.New("AMITIA_ADMIN_MYSQL_HOST 不能为空")
	}
	if c.MySQL.Port <= 0 || c.MySQL.Port > 65535 {
		return fmt.Errorf("AMITIA_ADMIN_MYSQL_PORT 无效: %d", c.MySQL.Port)
	}
	if strings.TrimSpace(c.MySQL.Username) == "" {
		return errors.New("AMITIA_ADMIN_MYSQL_USER 不能为空")
	}
	if strings.TrimSpace(c.MySQL.Password) == "" {
		return errors.New("AMITIA_ADMIN_MYSQL_PASSWORD 不能为空")
	}
	if strings.TrimSpace(c.MySQL.Database) == "" {
		return errors.New("AMITIA_ADMIN_MYSQL_DATABASE 不能为空")
	}
	if c.MySQL.MaxOpenConns <= 0 {
		return errors.New("AMITIA_ADMIN_MYSQL_MAX_OPEN_CONNS 必须大于 0")
	}
	if c.MySQL.MaxIdleConns < 0 || c.MySQL.MaxIdleConns > c.MySQL.MaxOpenConns {
		return errors.New("AMITIA_ADMIN_MYSQL_MAX_IDLE_CONNS 无效")
	}
	if c.SessionTTL <= 0 {
		return errors.New("管理会话有效期必须大于 0")
	}
	if c.MaxUploadBytes <= 0 {
		return errors.New("上传大小限制必须大于 0")
	}
	return nil
}

func detectProjectRoot() string {
	candidates := []string{}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, cwd)
	}
	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(executable))
	}
	for _, candidate := range candidates {
		current := candidate
		for {
			adminDir, adminErr := os.Stat(filepath.Join(current, "admin-system"))
			backendDir, backendErr := os.Stat(filepath.Join(current, "backend"))
			if adminErr == nil && backendErr == nil && adminDir.IsDir() && backendDir.IsDir() {
				return current
			}
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
			current = parent
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		return cwd
	}
	return "."
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt64(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envList(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			result = append(result, item)
		}
	}
	if len(result) == 0 {
		return fallback
	}
	return result
}
