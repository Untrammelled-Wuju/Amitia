// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package delivery

import (
	"fmt"

	"github.com/u-ai/backend/config"
)

const (
	defaultQQSidecarURL     = "http://127.0.0.1:19877"
	defaultWechatSidecarURL = "http://127.0.0.1:19876"
)

// BuildChannelResolverFromConfig constructs a ChannelResolver based on the
// application configuration. The web channel is always registered. QQ and
// Wechat channels are added only when the corresponding sidecar is enabled
// in config. Sidecar URLs default to the standard local ports unless the
// port value in config is non-zero.
func BuildChannelResolverFromConfig() ChannelResolver {
	adapters := []ChannelAdapter{
		NewWebChannelAdapter(),
	}

	cfg := config.AppCfg
	if cfg == nil {
		return NewMapChannelResolverWith(adapters)
	}

	if cfg.Runtime.Sidecars.QQ.Enabled {
		adapters = append(adapters, NewQQChannelAdapter(sidecarURL(cfg.Runtime.Sidecars.QQ.Port, defaultQQSidecarURL)))
	}
	if cfg.Runtime.Sidecars.Wechat.Enabled {
		adapters = append(adapters, NewWechatChannelAdapter(sidecarURL(cfg.Runtime.Sidecars.Wechat.Port, defaultWechatSidecarURL)))
	}

	return NewMapChannelResolverWith(adapters)
}

func sidecarURL(port int, defaultURL string) string {
	if port <= 0 {
		return defaultURL
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}
