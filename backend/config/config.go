package config

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)


type Config struct {
	Server  ServerConfig  `mapstructure:"server"`
	Storage StorageConfig `mapstructure:"storage"`
	JWT     JWTConfig     `mapstructure:"jwt"`
	App     AppConfig      `mapstructure:"app"`
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
	ExpireDays int    `mapstructure:"expireDays"`
}

type AppConfig struct {
	Name       string `mapstructure:"name"`
	Version    string `mapstructure:"version"`
	DeployMode string `mapstructure:"deployMode"`
}

var AppCfg *Config


func InitConfig(configPath string) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yml")
	v.AddConfigPath(configPath)
	v.AddConfigPath(".")

	
	v.SetDefault("server.port", 8900)
	v.SetDefault("server.host", "127.0.0.1")
	v.SetDefault("server.mode", "debug")
	v.SetDefault("storage.dataDir", "../data")
	v.SetDefault("jwt.secret", "u-ai-secret-key-change-me")
	v.SetDefault("jwt.expireDays", 7)
	v.SetDefault("app.name", "U-Ai")
	v.SetDefault("app.version", "1.0.0")
	v.SetDefault("app.deployMode", "desktop-local")

	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		fmt.Printf("[Config] 未找到配置文件，使用默认值: %v\n", err)
	} else {
		fmt.Printf("[Config] 已加载配置: %s\n", v.ConfigFileUsed())
	}

	AppCfg = &Config{}
	if err := v.Unmarshal(AppCfg); err != nil {
		log.Fatalf("配置解析失败: %v", err)
	}

	
	v.WatchConfig()
}


func (c *ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
