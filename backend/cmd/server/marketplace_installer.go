package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/agentplatform"
	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/runtimeprofile"
)

type marketplacePackageInstaller struct {
	runtime  *extension.Runtime
	dataDir  string
	profile  runtimeprofile.Profile
	client   *http.Client
	maxBytes int64
}

func newMarketplacePackageInstaller(runtime *extension.Runtime, dataDir string, profile runtimeprofile.Profile) *marketplacePackageInstaller {
	return &marketplacePackageInstaller{
		runtime: runtime, dataDir: dataDir, profile: profile,
		client: &http.Client{
			Timeout: 30 * time.Minute,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		maxBytes: 4 << 30,
	}
}

func (i *marketplacePackageInstaller) InstallMarketplaceListing(ctx context.Context, spaceID string, listing agentplatform.MarketplaceListing, version agentplatform.MarketplaceVersion) (string, error) {
	if i == nil || i.runtime == nil || i.runtime.Kernel == nil {
		return "", errors.New("插件安装器不可用")
	}
	packageURL := strings.TrimSpace(version.PackageURL)
	if packageURL == "" {
		return "", errors.New("插件市场版本缺少下载地址")
	}
	parsed, err := url.Parse(packageURL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http" && parsed.Scheme != "file") {
		return "", fmt.Errorf("插件市场下载地址无效: %s", packageURL)
	}
	var sourceReader io.ReadCloser
	if parsed.Scheme == "file" {
		if i.profile == runtimeprofile.ProfileCloudCore {
			return "", errors.New("Cloud Core 不允许直接从本机文件安装插件")
		}
		sourceReader, err = os.Open(filepath.FromSlash(parsed.Path))
		if err != nil {
			return "", fmt.Errorf("打开本地插件包: %w", err)
		}
	} else {
		if i.profile == runtimeprofile.ProfileCloudCore {
			host := parsed.Hostname()
			if parsed.Scheme != "https" {
				return "", errors.New("Cloud Core 只允许通过 HTTPS 下载市场插件")
			}
			if ip := net.ParseIP(host); ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()) {
				return "", errors.New("插件市场下载地址不能指向私有网络")
			}
			addresses, resolveErr := net.DefaultResolver.LookupIPAddr(ctx, host)
			if resolveErr != nil {
				return "", fmt.Errorf("解析插件市场下载主机: %w", resolveErr)
			}
			for _, address := range addresses {
				if address.IP.IsPrivate() || address.IP.IsLoopback() || address.IP.IsLinkLocalUnicast() || address.IP.IsUnspecified() {
					return "", errors.New("插件市场下载主机的 DNS 指向私有网络")
				}
			}
		}
		request, requestErr := http.NewRequestWithContext(ctx, http.MethodGet, packageURL, nil)
		if requestErr != nil {
			return "", fmt.Errorf("创建插件下载请求: %w", requestErr)
		}
		response, requestErr := i.client.Do(request)
		if requestErr != nil {
			return "", fmt.Errorf("下载插件包: %w", requestErr)
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			body, readErr := io.ReadAll(io.LimitReader(response.Body, 64<<10))
			response.Body.Close()
			if readErr != nil {
				return "", fmt.Errorf("插件下载失败 HTTP %d，读取响应失败: %w", response.StatusCode, readErr)
			}
			return "", fmt.Errorf("插件下载失败 HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
		}
		sourceReader = response.Body
	}
	defer sourceReader.Close()
	root := filepath.Join(i.dataDir, "agent-platform", "market-downloads")
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", fmt.Errorf("创建插件下载目录: %w", err)
	}
	file, err := os.CreateTemp(root, "plugin-*.amitiax")
	if err != nil {
		return "", fmt.Errorf("创建插件下载文件: %w", err)
	}
	name := file.Name()
	defer os.Remove(name)
	hash := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(sourceReader, i.maxBytes+1))
	closeErr := file.Close()
	if copyErr != nil {
		return "", fmt.Errorf("保存插件下载: %w", copyErr)
	}
	if closeErr != nil {
		return "", fmt.Errorf("关闭插件下载: %w", closeErr)
	}
	if size > i.maxBytes {
		return "", fmt.Errorf("插件包超过下载上限: %d", size)
	}
	expected := strings.ToLower(strings.TrimSpace(version.PackageHash))
	actual := hex.EncodeToString(hash.Sum(nil))
	if expected != "" && expected != actual {
		return "", fmt.Errorf("插件包 SHA-256 不匹配: expected=%s actual=%s", expected, actual)
	}
	source, err := os.Open(name)
	if err != nil {
		return "", fmt.Errorf("打开插件下载文件: %w", err)
	}
	defer source.Close()
	allowUnsigned := i.profile != runtimeprofile.ProfileCloudCore
	preview, err := i.runtime.Kernel.PreviewPackage(ctx, kernel.PackagePreviewRequest{
		SpaceID: spaceID, ScopeType: "global", FileName: version.OriginalName,
		AllowUnsignedLocal: allowUnsigned, AllowUnsignedDev: false,
	}, source)
	if err != nil {
		return "", fmt.Errorf("预览插件市场包: %w", err)
	}
	if !preview.Installable {
		messages := make([]string, 0, len(preview.Issues))
		for _, issue := range preview.Issues {
			messages = append(messages, issue.Message)
		}
		return "", fmt.Errorf("插件市场包不可安装: %s", strings.Join(messages, "；"))
	}
	if preview.ExtensionID != listing.ExtensionID {
		return "", fmt.Errorf("插件标识不匹配: expected=%s actual=%s", listing.ExtensionID, preview.ExtensionID)
	}
	if preview.Version != version.Version {
		return "", fmt.Errorf("插件版本不匹配: expected=%s actual=%s", version.Version, preview.Version)
	}
	confirmations := make(map[string]bool, len(preview.RequiredConfirmations))
	for _, item := range preview.RequiredConfirmations {
		confirmations[item] = true
	}
	confirmation, err := i.runtime.Kernel.ConfirmPackagePreview(ctx, kernel.PackagePreviewConfirmationRequest{
		SessionID: preview.SessionID, SpaceID: spaceID, ScopeType: "global", Confirmations: confirmations,
	})
	if err != nil {
		return "", fmt.Errorf("确认插件市场包: %w", err)
	}
	result, err := i.runtime.Kernel.ExecutePackageInstall(ctx, kernel.PackageInstallRequest{
		SessionID: preview.SessionID, SpaceID: spaceID, ScopeType: "global",
		Confirmations: confirmations, ConfirmationToken: confirmation.ConfirmationToken,
		ExpectedExtensionID: listing.ExtensionID,
		IdempotencyKey:      "market:" + listing.ExtensionID + ":" + version.Version + ":" + spaceID,
	})
	if err != nil {
		return "", fmt.Errorf("安装插件市场包: %w", err)
	}
	return result.OperationID, nil
}
