package system

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func (s *service) AdministratorSettingsContext(ctx context.Context, operation string, body map[string]interface{}) (map[string]interface{}, error) {
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return nil, err
	}
	settings := map[string]string{}
	setBool := func(input, key string) {
		if value, ok := body[input].(bool); ok {
			settings[key] = strconv.FormatBool(value)
		}
	}
	switch operation {
	case "audit":
		setBool("enabled", "audit_enabled")
		setBool("logActions", "audit_log_actions")
		if value, ok := body["retentionDays"].(float64); ok {
			days := int(value)
			if days < 1 {
				days = 1
			} else if days > 3650 {
				days = 3650
			}
			settings["audit_retention_days"] = strconv.Itoa(days)
		}
	case "security":
		setBool("requireAuth", "require_auth")
		setBool("rateLimit", "rate_limit")
		if value, ok := body["allowedOrigins"].(string); ok {
			settings["allowed_origins"] = value
		}
	case "runtime":
		if _, frozen := body["runtimeProfile"]; !frozen {
			mode, _ := body["deployMode"].(string)
			if mode == "" {
				mode, _ = body["mode"].(string)
			}
			if mode == "desktop-local" || mode == "cloud-web" {
				settings["runtime_mode"] = mode
				if mode == "cloud-web" {
					settings["require_auth"] = "true"
				}
			}
			if value, ok := body["publicBaseUrl"].(string); ok {
				settings["public_base_url"] = strings.TrimSpace(value)
			}
		}
	case "long-running":
		for input, key := range map[string]string{"maxTasks": "long_running_max_tasks", "timeoutMinutes": "long_running_timeout"} {
			if value, ok := body[input]; ok {
				settings[key] = strconv.Itoa(toInt(value))
			}
		}
	case "update":
		setBool("autoCheck", "auto_update")
	case "setup-finish", "onboarding-complete":
		settings = map[string]string{"setup_completed": "true", "onboarding_completed": "true", "setup_step": "done"}
	case "setup-reset", "onboarding-reset":
		settings = map[string]string{"setup_completed": "false", "onboarding_completed": "false", "setup_step": ""}
	case "setup-step":
		step, _ := body["step"].(string)
		settings["setup_step"] = step
	case "release-check":
		settings["last_release_check"] = time.Now().Format(time.DateTime)
	case "reload":
		data, err := os.ReadFile(filepath.Join("..", "appsettings.json"))
		if err != nil {
			return nil, err
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal(data, &parsed); err != nil {
			return nil, err
		}
		settings["config_last_reload"] = time.Now().Format(time.DateTime)
	default:
		return nil, errors.New("未知管理员配置操作")
	}
	current, err := s.applyAppSettings(ctx, settings)
	if err != nil {
		return nil, err
	}
	text := func(key, fallback string) string {
		value, _ := current[key].(string)
		if value == "" {
			return fallback
		}
		return value
	}
	switch operation {
	case "audit":
		days, err := strconv.Atoi(text("audit_retention_days", "90"))
		if err != nil || days < 1 || days > 3650 {
			days = 90
		}
		return map[string]interface{}{"enabled": text("audit_enabled", "true") != "false", "logActions": text("audit_log_actions", "true") != "false", "retentionDays": days}, nil
	case "security":
		return map[string]interface{}{"requireAuth": text("require_auth", "true") != "false", "allowedOrigins": text("allowed_origins", "*"), "rateLimit": text("rate_limit", "true") != "false"}, nil
	case "runtime":
		mode := text("runtime_mode", "desktop-local")
		host := "127.0.0.1"
		if mode == "cloud-web" {
			host = "0.0.0.0"
		}
		return map[string]interface{}{"mode": mode, "deployMode": mode, "runtimeProfile": s.runtimeProfile.String(), "host": host, "port": envInt("AMITIA_SERVER_PORT", 18080), "web": map[string]interface{}{"enabled": true, "publicBaseUrl": text("public_base_url", ""), "requireAuth": text("require_auth", "true") != "false"}, "storage": map[string]interface{}{"dataDir": s.dataDir}, "requiresRestart": true}, nil
	case "long-running":
		return map[string]interface{}{"maxTasks": toInt(text("long_running_max_tasks", "5")), "timeoutMinutes": toInt(text("long_running_timeout", "30"))}, nil
	case "update":
		return map[string]interface{}{"autoCheck": text("auto_update", "true") != "false", "channel": "stable", "lastCheckAt": nil}, nil
	case "setup-finish":
		return map[string]interface{}{"finished": true}, nil
	case "setup-reset", "onboarding-reset":
		return map[string]interface{}{"reset": true}, nil
	case "setup-step":
		return map[string]interface{}{"currentStep": text("setup_step", ""), "done": false}, nil
	case "onboarding-complete":
		return map[string]interface{}{"completed": true}, nil
	case "release-check":
		return s.GetReleaseCheckLatest(), nil
	case "reload":
		return map[string]interface{}{"reloaded": true, "reloadedAt": text("config_last_reload", "")}, nil
	}
	return nil, errors.New("未知管理员配置响应")
}
