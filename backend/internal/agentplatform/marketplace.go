package agentplatform

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/u-ai/backend/internal/extension/kernel/packageverify"
	"github.com/u-ai/backend/internal/extension/kernel/trust"
)

type MarketplaceInstaller interface {
	InstallMarketplaceListing(ctx context.Context, spaceID string, listing MarketplaceListing, version MarketplaceVersion) (string, error)
}

type PublishMarketplaceListingInput struct {
	ExtensionID   string   `json:"extensionId"`
	PublisherID   string   `json:"publisherId"`
	CategoryID    string   `json:"categoryId"`
	DisplayName   string   `json:"displayName"`
	Summary       string   `json:"summary"`
	Description   string   `json:"description"`
	Icon          string   `json:"icon"`
	Version       string   `json:"version"`
	TrustLevel    string   `json:"trustLevel"`
	SecurityScore int      `json:"securityScore"`
	Platforms     []string `json:"platforms"`
	Capabilities  []string `json:"capabilities"`
	Permissions   []string `json:"permissions"`
	Tags          []string `json:"tags"`
	PackageURL    string   `json:"packageUrl"`
	PackageHash   string   `json:"packageHash"`
	Signature     string   `json:"signature"`
	SBOM          any      `json:"sbom"`
	Compatibility any      `json:"compatibility"`
}

type MarketplaceFilter struct {
	CategoryID string
	Search     string
	TrustLevel string
	Platform   string
	Publisher  string
	Limit      int
}

type MarketplaceListingView struct {
	Listing     MarketplaceListing   `json:"listing"`
	Publisher   MarketplacePublisher `json:"publisher"`
	Versions    []MarketplaceVersion `json:"versions"`
	Category    MarketplaceCategory  `json:"category"`
	Installable bool                 `json:"installable"`
}

type remoteMarketplaceCatalog struct {
	Categories []remoteMarketplaceCategory    `json:"categories"`
	Packages   []remoteMarketplacePackageView `json:"packages"`
}

type remoteMarketplaceCategory struct {
	ID          uint   `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	SortOrder   int    `json:"sortOrder"`
}

type remoteMarketplacePackageView struct {
	ID            uint                      `json:"id"`
	ExtensionID   string                    `json:"extensionId"`
	CategoryID    uint                      `json:"categoryId"`
	DisplayName   string                    `json:"displayName"`
	Summary       string                    `json:"summary"`
	Description   string                    `json:"description"`
	PublisherID   string                    `json:"publisherId"`
	TrustLevel    string                    `json:"trustLevel"`
	SecurityScore int                       `json:"securityScore"`
	Status        string                    `json:"status"`
	LatestVersion string                    `json:"latestVersion"`
	Icon          string                    `json:"icon"`
	Tags          []string                  `json:"tags"`
	Category      remoteMarketplaceCategory `json:"category"`
	Versions      []remoteMarketVersion     `json:"versions"`
}

type remoteMarketVersion struct {
	ID                uint       `json:"id"`
	PackageID         uint       `json:"packageId"`
	Version           string     `json:"version"`
	OriginalName      string     `json:"originalName"`
	Size              int64      `json:"size"`
	SHA256            string     `json:"sha256"`
	Signature         string     `json:"signature"`
	SBOMJSON          string     `json:"sbomJson"`
	CompatibilityJSON string     `json:"compatibilityJson"`
	ReleaseNotes      string     `json:"releaseNotes"`
	Status            string     `json:"status"`
	PublishedAt       *time.Time `json:"publishedAt"`
}

func (s *Service) SetMarketplaceInstaller(installer MarketplaceInstaller) {
	s.marketplaceInstaller = installer
}

func (s *Service) SeedMarketplace(ctx context.Context) error {
	categories := []MarketplaceCategory{
		{ID: "agent-identities", Name: "Agent 身份", Description: "邮箱、日历、手机号、浏览器身份和数字签名", Icon: "identity", SortOrder: 10},
		{ID: "productivity", Name: "效率工具", Description: "邮件、日程、文档和任务管理", Icon: "productivity", SortOrder: 20},
		{ID: "communication", Name: "沟通协作", Description: "消息、渠道、联系人和团队协作", Icon: "communication", SortOrder: 30},
		{ID: "developer", Name: "开发工具", Description: "代码、Git、终端、CI 和开发者服务", Icon: "developer", SortOrder: 40},
		{ID: "computer", Name: "电脑与自动化", Description: "浏览器、桌面、终端和设备自动化", Icon: "computer", SortOrder: 50},
		{ID: "knowledge", Name: "知识与数据", Description: "知识库、搜索、分析和数据连接", Icon: "knowledge", SortOrder: 60},
		{ID: "finance", Name: "财务与资产", Description: "账单、支付、钱包和资产操作", Icon: "finance", SortOrder: 70},
		{ID: "lifestyle", Name: "生活与陪伴", Description: "生活、情绪、关系和个人服务", Icon: "lifestyle", SortOrder: 80},
		{ID: "channels", Name: "消息渠道", Description: "微信、QQ、Telegram 和企业消息", Icon: "channels", SortOrder: 90},
		{ID: "security", Name: "安全与治理", Description: "认证、审计、策略和合规工具", Icon: "security", SortOrder: 100},
	}
	now := time.Now().UTC()
	for index := range categories {
		categories[index].CreatedAt = now
		if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&categories[index]).Error; err != nil {
			return fmt.Errorf("初始化插件市场分类: %w", err)
		}
	}
	publisher := MarketplacePublisher{ID: "com.amitia", DisplayName: "Amitia", TrustLevel: "official", Homepage: "https://amitia.untrammelled.top", Status: "active", CreatedAt: now, UpdatedAt: now}
	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&publisher).Error; err != nil {
		return fmt.Errorf("初始化插件市场发布者: %w", err)
	}
	return nil
}

func (s *Service) SeedLocalPluginMarketplace(ctx context.Context, root string) error {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取本地插件市场目录: %w", err)
	}
	now := time.Now().UTC()
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".amitiax") {
			continue
		}
		path, err := filepath.Abs(filepath.Join(root, entry.Name()))
		if err != nil {
			return fmt.Errorf("解析本地插件路径: %w", err)
		}
		manifest, err := readPackageManifest(path)
		if err != nil {
			return err
		}
		packageHash, err := fileSHA256(path)
		if err != nil {
			return err
		}
		signature, err := readPackageSignature(path)
		if err != nil {
			return err
		}
		extensionID := stringValue(manifest, "extension", "id")
		version := stringValue(manifest, "extension", "version")
		if extensionID == "" || version == "" {
			continue
		}
		name := stringValue(manifest, "extension", "name", "default")
		description := stringValue(manifest, "extension", "description", "default")
		publisherID := stringValue(manifest, "publisher", "id")
		trustLevel := stringValue(manifest, "publisher", "trustLevel")
		if trustLevel == "official" {
			if publisherID != trust.OfficialPublisherID {
				trustLevel = "community"
			} else if err := packageverify.VerifyOfficialPublisherPackage(ctx, path); err != nil {
				return fmt.Errorf("验证官方插件包 %s: %w", entry.Name(), err)
			}
		}
		listingID := stableID("marketlisting", extensionID)
		listing := MarketplaceListing{
			ID: listingID, ExtensionID: extensionID, PublisherID: firstNonEmpty(publisherID, "com.amitia"),
			CategoryID: "agent-identities", DisplayName: firstNonEmpty(name, extensionID),
			Summary: description, Description: description, LatestVersion: version,
			TrustLevel: firstNonEmpty(trustLevel, "official"), SecurityScore: 90,
			PlatformsJSON: "[]", CapabilitiesJSON: "[]", PermissionsJSON: "[]",
			Status: "published", PublishedAt: &now, CreatedAt: now, UpdatedAt: now,
		}
		versionRecord := MarketplaceVersion{
			ID: stableID("marketversion", extensionID+":"+version), ListingID: listingID,
			Version: version, OriginalName: entry.Name(), PackageURL: "file://" + filepath.ToSlash(path),
			PackageHash: packageHash, Signature: signature,
			SBOMJSON: "{}", CompatibilityJSON: "{}", Status: "published", CreatedAt: now, UpdatedAt: now,
		}
		if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var existing MarketplaceListing
			err := tx.Where("extension_id = ?", extensionID).First(&existing).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := tx.Create(&listing).Error; err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else {
				listing.ID = existing.ID
				versionRecord.ListingID = existing.ID
				if err := tx.Model(&MarketplaceListing{}).Where("id = ?", existing.ID).Updates(map[string]any{
					"display_name": listing.DisplayName, "summary": listing.Summary,
					"description": listing.Description, "latest_version": version,
					"category_id": "agent-identities", "status": "published", "updated_at": now,
				}).Error; err != nil {
					return err
				}
			}
			return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "listing_id"}, {Name: "version"}}, DoUpdates: clause.AssignmentColumns([]string{"original_name", "package_url", "package_hash", "signature", "status", "updated_at"})}).Create(&versionRecord).Error
		}); err != nil {
			return fmt.Errorf("同步本地插件市场包 %s: %w", extensionID, err)
		}
	}
	return nil
}

func readPackageManifest(path string) (map[string]any, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("打开本地插件包 %s: %w", path, err)
	}
	defer archive.Close()
	for _, file := range archive.File {
		if file.Name != "manifest.json" {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("读取本地插件清单: %w", err)
		}
		body, err := io.ReadAll(io.LimitReader(reader, 4<<20))
		reader.Close()
		if err != nil {
			return nil, fmt.Errorf("读取本地插件清单: %w", err)
		}
		var manifest map[string]any
		if err := json.Unmarshal(body, &manifest); err != nil {
			return nil, fmt.Errorf("解析本地插件清单: %w", err)
		}
		return manifest, nil
	}
	return nil, fmt.Errorf("本地插件包缺少 manifest.json: %s", path)
}

func readPackageSignature(path string) (string, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("打开本地插件包签名: %w", err)
	}
	defer archive.Close()
	for _, file := range archive.File {
		if file.Name != "META-INF/amitia-signature.json" {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			return "", fmt.Errorf("读取本地插件包签名: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(reader, 1<<20))
		closeErr := reader.Close()
		if readErr != nil {
			return "", fmt.Errorf("读取本地插件包签名: %w", readErr)
		}
		if closeErr != nil {
			return "", fmt.Errorf("关闭本地插件包签名: %w", closeErr)
		}
		return strings.TrimSpace(string(body)), nil
	}
	return "", nil
}

func stringValue(root map[string]any, keys ...string) string {
	var current any = root
	for _, key := range keys {
		value, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = value[key]
	}
	value, _ := current.(string)
	return strings.TrimSpace(value)
}

func (s *Service) ListMarketplaceCategories(ctx context.Context) ([]MarketplaceCategory, error) {
	if err := s.SyncRemoteMarketplace(ctx); err != nil {
		return nil, err
	}
	categories := make([]MarketplaceCategory, 0)
	if err := s.db.WithContext(ctx).Order("sort_order ASC, name ASC").Find(&categories).Error; err != nil {
		return nil, fmt.Errorf("读取插件市场分类: %w", err)
	}
	return categories, nil
}

func (s *Service) PublishMarketplaceListing(ctx context.Context, input PublishMarketplaceListingInput) (MarketplaceListing, error) {
	if strings.TrimSpace(input.ExtensionID) == "" || strings.TrimSpace(input.DisplayName) == "" || strings.TrimSpace(input.Version) == "" {
		return MarketplaceListing{}, errors.New("插件 ID、名称和版本不能为空")
	}
	if strings.TrimSpace(input.CategoryID) == "" {
		input.CategoryID = "productivity"
	}
	var category MarketplaceCategory
	if err := s.db.WithContext(ctx).Where("id = ?", input.CategoryID).First(&category).Error; err != nil {
		return MarketplaceListing{}, fmt.Errorf("插件市场分类不存在: %w", err)
	}
	if strings.TrimSpace(input.PublisherID) == "" {
		input.PublisherID = "com.amitia"
	}
	trustLevel := strings.ToLower(strings.TrimSpace(input.TrustLevel))
	if trustLevel == "" {
		trustLevel = "community"
	}
	if trustLevel != "official" && trustLevel != "verified" && trustLevel != "trusted" && trustLevel != "community" {
		return MarketplaceListing{}, fmt.Errorf("不支持的插件信任等级: %s", trustLevel)
	}
	if trustLevel == "official" && input.PublisherID != "com.amitia" {
		return MarketplaceListing{}, errors.New("只有 com.amitia 可以发布官方插件")
	}
	if decoded, err := hex.DecodeString(strings.TrimSpace(input.PackageHash)); err != nil || len(decoded) != sha256.Size {
		return MarketplaceListing{}, errors.New("插件市场发布必须提供 SHA-256 包哈希")
	}
	if (trustLevel == "official" || trustLevel == "verified") && strings.TrimSpace(input.Signature) == "" {
		return MarketplaceListing{}, fmt.Errorf("%s 插件必须提供发布签名", trustLevel)
	}
	sbomObject, ok := input.SBOM.(map[string]any)
	if !ok || sbomObject == nil {
		return MarketplaceListing{}, errors.New("插件市场发布必须提供 SBOM 对象")
	}
	publisher := MarketplacePublisher{ID: input.PublisherID, DisplayName: input.PublisherID, TrustLevel: trustLevel, Status: "active", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&publisher).Error; err != nil {
		return MarketplaceListing{}, fmt.Errorf("创建插件市场发布者: %w", err)
	}
	platformsJSON, _, err := hashJSON(input.Platforms)
	if err != nil {
		return MarketplaceListing{}, err
	}
	capabilitiesJSON, _, err := hashJSON(input.Capabilities)
	if err != nil {
		return MarketplaceListing{}, err
	}
	permissionsJSON, _, err := hashJSON(input.Permissions)
	if err != nil {
		return MarketplaceListing{}, err
	}
	tagsJSON, _, err := hashJSON(input.Tags)
	if err != nil {
		return MarketplaceListing{}, err
	}
	sbomJSON, _, err := hashJSON(input.SBOM)
	if err != nil {
		return MarketplaceListing{}, err
	}
	compatibilityJSON, _, err := hashJSON(input.Compatibility)
	if err != nil {
		return MarketplaceListing{}, err
	}
	now := time.Now().UTC()
	var listing MarketplaceListing
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("extension_id = ?", input.ExtensionID).First(&listing).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			listing = MarketplaceListing{
				ID: newID("marketlisting"), ExtensionID: input.ExtensionID, PublisherID: input.PublisherID,
				CategoryID: input.CategoryID, TagsJSON: tagsJSON, DisplayName: input.DisplayName,
				Summary: input.Summary, Description: input.Description, Icon: input.Icon,
				LatestVersion: input.Version, TrustLevel: trustLevel,
				SecurityScore: input.SecurityScore, PlatformsJSON: platformsJSON, CapabilitiesJSON: capabilitiesJSON,
				PermissionsJSON: permissionsJSON, Status: "published", PublishedAt: &now,
				CreatedAt: now, UpdatedAt: now,
			}
			if err := tx.Create(&listing).Error; err != nil {
				return fmt.Errorf("创建插件市场条目: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("读取插件市场条目: %w", err)
		} else {
			if err := tx.Model(&MarketplaceListing{}).Where("id = ?", listing.ID).Updates(map[string]any{
				"publisher_id": input.PublisherID, "category_id": input.CategoryID, "tags_json": tagsJSON,
				"display_name": input.DisplayName, "summary": input.Summary, "description": input.Description,
				"icon": input.Icon, "latest_version": input.Version, "trust_level": trustLevel,
				"security_score": input.SecurityScore, "platforms_json": platformsJSON,
				"capabilities_json": capabilitiesJSON, "permissions_json": permissionsJSON,
				"status": "published", "published_at": now, "withdrawn_at": nil, "updated_at": now,
			}).Error; err != nil {
				return fmt.Errorf("更新插件市场条目: %w", err)
			}
		}
		version := MarketplaceVersion{
			ID: newID("marketversion"), ListingID: listing.ID, Version: input.Version,
			PackageURL: input.PackageURL, PackageHash: input.PackageHash, Signature: input.Signature,
			SBOMJSON: sbomJSON, CompatibilityJSON: compatibilityJSON, Status: "published", CreatedAt: now,
		}
		createdVersion := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&version)
		if createdVersion.Error != nil {
			return fmt.Errorf("创建插件市场版本: %w", createdVersion.Error)
		}
		if createdVersion.RowsAffected != 1 {
			return fmt.Errorf("插件市场版本已存在: %s", input.Version)
		}
		return nil
	})
	if err != nil {
		return MarketplaceListing{}, err
	}
	if err := s.db.WithContext(ctx).Where("extension_id = ?", input.ExtensionID).First(&listing).Error; err != nil {
		return MarketplaceListing{}, fmt.Errorf("读取插件市场条目: %w", err)
	}
	return listing, nil
}

func (s *Service) WithdrawMarketplaceVersion(ctx context.Context, listingID, version, reason string) error {
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&MarketplaceVersion{}).Where("listing_id = ? AND version = ? AND status = ?", listingID, version, "published").Updates(map[string]any{"status": "withdrawn", "withdrawn_at": now})
		if result.Error != nil {
			return fmt.Errorf("撤回插件版本: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return errors.New("插件版本不存在或已经撤回")
		}
		var listing MarketplaceListing
		if err := tx.Where("id = ?", listingID).First(&listing).Error; err != nil {
			return fmt.Errorf("读取插件条目: %w", err)
		}
		if listing.LatestVersion == version {
			return tx.Model(&MarketplaceListing{}).Where("id = ?", listingID).Updates(map[string]any{"status": "withdrawn", "withdrawn_at": now, "updated_at": now}).Error
		}
		return nil
	})
}

func (s *Service) ListMarketplace(ctx context.Context, filter MarketplaceFilter) ([]MarketplaceListingView, error) {
	if err := s.SyncRemoteMarketplace(ctx); err != nil {
		return nil, err
	}
	query := s.db.WithContext(ctx).Where("status = ?", "published")
	if strings.TrimSpace(filter.CategoryID) != "" {
		query = query.Where("category_id = ?", filter.CategoryID)
	}
	if strings.TrimSpace(filter.TrustLevel) != "" {
		query = query.Where("trust_level = ?", filter.TrustLevel)
	}
	if strings.TrimSpace(filter.Publisher) != "" {
		query = query.Where("publisher_id = ?", filter.Publisher)
	}
	search := strings.ToLower(strings.TrimSpace(filter.Search))
	if search != "" {
		like := "%" + search + "%"
		query = query.Where("LOWER(display_name) LIKE ? OR LOWER(summary) LIKE ? OR LOWER(description) LIKE ? OR LOWER(extension_id) LIKE ?", like, like, like, like)
	}
	listings := make([]MarketplaceListing, 0)
	if err := query.Order("install_count DESC, rating_average DESC, updated_at DESC").Limit(trimLimit(filter.Limit, 100, 500)).Find(&listings).Error; err != nil {
		return nil, fmt.Errorf("读取插件市场: %w", err)
	}
	platform := strings.ToLower(strings.TrimSpace(filter.Platform))
	result := make([]MarketplaceListingView, 0, len(listings))
	for _, listing := range listings {
		if platform != "" {
			var platforms []string
			if err := json.Unmarshal([]byte(listing.PlatformsJSON), &platforms); err != nil {
				return nil, fmt.Errorf("解析插件平台: %w", err)
			}
			if len(platforms) > 0 && !containsString(platforms, platform) {
				continue
			}
		}
		view, err := s.marketplaceView(ctx, listing)
		if err != nil {
			return nil, err
		}
		result = append(result, view)
	}
	sort.SliceStable(result, func(i, j int) bool {
		left, right := trustRank(result[i].Listing.TrustLevel), trustRank(result[j].Listing.TrustLevel)
		if left != right {
			return left > right
		}
		if result[i].Listing.SecurityScore != result[j].Listing.SecurityScore {
			return result[i].Listing.SecurityScore > result[j].Listing.SecurityScore
		}
		return result[i].Listing.InstallCount > result[j].Listing.InstallCount
	})
	return result, nil
}

func (s *Service) GetMarketplaceListing(ctx context.Context, listingID string) (MarketplaceListingView, error) {
	var listing MarketplaceListing
	if err := s.db.WithContext(ctx).Where("id = ? OR extension_id = ?", listingID, listingID).First(&listing).Error; err != nil {
		return MarketplaceListingView{}, fmt.Errorf("读取插件市场条目: %w", err)
	}
	return s.marketplaceView(ctx, listing)
}

func (s *Service) InstallMarketplaceListing(ctx context.Context, spaceID, listingID, version, actorID string) (MarketplaceInstallation, error) {
	var listing MarketplaceListing
	if err := s.db.WithContext(ctx).Where("id = ? OR extension_id = ?", listingID, listingID).First(&listing).Error; err != nil {
		return MarketplaceInstallation{}, fmt.Errorf("读取插件市场条目: %w", err)
	}
	if listing.Status != "published" {
		return MarketplaceInstallation{}, errors.New("插件条目未发布")
	}
	if strings.TrimSpace(version) == "" {
		version = listing.LatestVersion
	}
	var selected MarketplaceVersion
	if err := s.db.WithContext(ctx).Where("listing_id = ? AND version = ? AND status = ?", listing.ID, version, "published").First(&selected).Error; err != nil {
		return MarketplaceInstallation{}, fmt.Errorf("读取插件市场版本: %w", err)
	}
	if s.marketplaceInstaller == nil {
		return MarketplaceInstallation{}, errors.New("插件安装器不可用")
	}
	operationID, err := s.marketplaceInstaller.InstallMarketplaceListing(ctx, spaceID, listing, selected)
	if err != nil {
		return MarketplaceInstallation{}, fmt.Errorf("安装插件市场版本: %w", err)
	}
	now := time.Now().UTC()
	installation := MarketplaceInstallation{ID: newID("marketinstall"), SpaceID: spaceID, ListingID: listing.ID, Version: version, Status: "requested", OperationID: operationID, RequestedBy: actorID, CreatedAt: now, UpdatedAt: now}
	if err := s.db.WithContext(ctx).Create(&installation).Error; err != nil {
		return MarketplaceInstallation{}, fmt.Errorf("记录插件安装: %w", err)
	}
	if err := s.db.WithContext(ctx).Model(&MarketplaceListing{}).Where("id = ?", listing.ID).UpdateColumn("install_count", gorm.Expr("install_count + 1")).Error; err != nil {
		return MarketplaceInstallation{}, fmt.Errorf("更新插件市场安装次数: %w", err)
	}
	if err := s.recordActivity(ctx, ActivityEvent{SpaceID: spaceID, Kind: "marketplace.install", State: "requested", Title: "插件安装已提交", Detail: listing.DisplayName + " " + version}); err != nil {
		return MarketplaceInstallation{}, err
	}
	return installation, nil
}

func (s *Service) marketplaceView(ctx context.Context, listing MarketplaceListing) (MarketplaceListingView, error) {
	view := MarketplaceListingView{Listing: listing, Installable: listing.Status == "published"}
	if err := s.db.WithContext(ctx).Where("id = ?", listing.PublisherID).First(&view.Publisher).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return MarketplaceListingView{}, fmt.Errorf("读取插件发布者: %w", err)
	}
	if err := s.db.WithContext(ctx).Where("id = ?", listing.CategoryID).First(&view.Category).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return MarketplaceListingView{}, fmt.Errorf("读取插件分类: %w", err)
	}
	if err := s.db.WithContext(ctx).Where("listing_id = ?", listing.ID).Order("created_at DESC").Find(&view.Versions).Error; err != nil {
		return MarketplaceListingView{}, fmt.Errorf("读取插件版本: %w", err)
	}
	return view, nil
}

func (s *Service) SyncRemoteMarketplace(ctx context.Context) error {
	if s == nil || strings.TrimSpace(s.remoteMarketURL) == "" {
		return nil
	}
	s.remoteMarketMu.Lock()
	defer s.remoteMarketMu.Unlock()
	if !s.remoteMarketSynced.IsZero() && time.Since(s.remoteMarketSynced) < 30*time.Second {
		return nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, s.remoteMarketURL, nil)
	if err != nil {
		return fmt.Errorf("创建插件市场同步请求: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	response, err := s.remoteMarketClient.Do(request)
	if err != nil {
		return fmt.Errorf("连接插件市场: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return fmt.Errorf("读取插件市场响应: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("插件市场返回 HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	var envelope struct {
		Code int                      `json:"code"`
		Msg  string                   `json:"msg"`
		Data remoteMarketplaceCatalog `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("解析插件市场响应: %w", err)
	}
	if envelope.Code != 0 && envelope.Code != 200 {
		return fmt.Errorf("插件市场返回错误 %d: %s", envelope.Code, envelope.Msg)
	}
	now := time.Now().UTC()
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, category := range envelope.Data.Categories {
			if strings.TrimSpace(category.Slug) == "" {
				continue
			}
			existing := MarketplaceCategory{ID: category.Slug}
			if err := tx.Where("id = ?", category.Slug).First(&existing).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("读取远程插件分类: %w", err)
			}
			if strings.TrimSpace(existing.ID) == "" {
				existing.CreatedAt = now
			}
			existing.Name = category.Name
			existing.Description = category.Description
			existing.Icon = category.Icon
			existing.SortOrder = category.SortOrder
			if err := tx.Save(&existing).Error; err != nil {
				return fmt.Errorf("同步远程插件分类: %w", err)
			}
		}
		for _, remotePackage := range envelope.Data.Packages {
			publisherID := firstNonEmpty(remotePackage.PublisherID, "com.amitia")
			trustLevel := firstNonEmpty(remotePackage.TrustLevel, "community")
			if trustLevel == "official" && publisherID != "com.amitia" {
				trustLevel = "community"
			}
			publisher := MarketplacePublisher{ID: publisherID, DisplayName: publisherID, TrustLevel: trustLevel, Status: "active", CreatedAt: now, UpdatedAt: now}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&publisher).Error; err != nil {
				return fmt.Errorf("同步远程插件发布者: %w", err)
			}
			listingID := stableID("marketlisting", remotePackage.ExtensionID)
			var listing MarketplaceListing
			err := tx.Where("extension_id = ?", remotePackage.ExtensionID).First(&listing).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("读取远程插件市场条目: %w", err)
			}
			if strings.TrimSpace(listing.ID) == "" {
				listing.ID = listingID
				listing.CreatedAt = now
			}
			tagsJSON, _, err := hashJSON(remotePackage.Tags)
			if err != nil {
				return err
			}
			listing.PublisherID = publisherID
			listing.CategoryID = remotePackage.Category.Slug
			listing.TagsJSON = tagsJSON
			listing.DisplayName = remotePackage.DisplayName
			listing.Summary = remotePackage.Summary
			listing.Description = remotePackage.Description
			listing.Icon = remotePackage.Icon
			listing.LatestVersion = remotePackage.LatestVersion
			listing.TrustLevel = trustLevel
			listing.SecurityScore = remotePackage.SecurityScore
			listing.PlatformsJSON = "[]"
			listing.CapabilitiesJSON = "[]"
			listing.PermissionsJSON = "[]"
			listing.Status = firstNonEmpty(remotePackage.Status, "published")
			listing.PublishedAt = &now
			listing.UpdatedAt = now
			if err := tx.Save(&listing).Error; err != nil {
				return fmt.Errorf("同步远程插件市场条目: %w", err)
			}
			for _, remoteVersion := range remotePackage.Versions {
				if remoteVersion.Status != "published" && remoteVersion.Status != "superseded" {
					continue
				}
				if err := validateRemoteMarketplaceVersion(trustLevel, remoteVersion); err != nil {
					return fmt.Errorf("远程插件 %s %s 未通过可信校验: %w", remotePackage.ExtensionID, remoteVersion.Version, err)
				}
				versionID := stableID("marketversion", fmt.Sprintf("%s:%s", remotePackage.ExtensionID, remoteVersion.Version))
				var local MarketplaceVersion
				err := tx.Where("listing_id = ? AND version = ?", listing.ID, remoteVersion.Version).First(&local).Error
				if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					return fmt.Errorf("读取远程插件版本: %w", err)
				}
				if strings.TrimSpace(local.ID) == "" {
					local.ID = versionID
					local.CreatedAt = now
				}
				local.ListingID = listing.ID
				local.Version = remoteVersion.Version
				local.OriginalName = remoteVersion.OriginalName
				local.PackageURL = remoteDownloadURL(s.remoteMarketURL, remotePackage.ID, remoteVersion.Version)
				local.PackageHash = remoteVersion.SHA256
				local.Signature = remoteVersion.Signature
				local.SBOMJSON = firstNonEmpty(remoteVersion.SBOMJSON, "{}")
				local.CompatibilityJSON = firstNonEmpty(remoteVersion.CompatibilityJSON, "{}")
				local.Status = remoteVersion.Status
				local.UpdatedAt = now
				if err := tx.Save(&local).Error; err != nil {
					return fmt.Errorf("同步远程插件版本: %w", err)
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}
	s.remoteMarketSynced = now
	return nil
}

func remoteDownloadURL(catalogURL string, packageID uint, version string) string {
	base := strings.TrimRight(strings.TrimSpace(catalogURL), "/")
	base = strings.TrimSuffix(base, "/catalog")
	return fmt.Sprintf("%s/packages/%d/download?version=%s", base, packageID, version)
}

func trustRank(value string) int {
	switch value {
	case "platform":
		return 6
	case "official":
		return 5
	case "verified":
		return 4
	case "trusted":
		return 3
	case "user_trusted":
		return 2
	case "community":
		return 1
	default:
		return 0
	}
}

func containsString(values []string, value string) bool {
	for _, item := range values {
		if strings.EqualFold(item, value) {
			return true
		}
	}
	return false
}

func validateRemoteMarketplaceVersion(trustLevel string, version remoteMarketVersion) error {
	decoded, err := hex.DecodeString(strings.TrimSpace(version.SHA256))
	if err != nil || len(decoded) != sha256.Size {
		return errors.New("缺少有效 SHA-256")
	}
	if (trustLevel == "official" || trustLevel == "verified" || trustLevel == "trusted") && strings.TrimSpace(version.Signature) == "" {
		return errors.New("缺少发布签名")
	}
	var sbom map[string]any
	if err := json.Unmarshal([]byte(version.SBOMJSON), &sbom); err != nil {
		return fmt.Errorf("SBOM 无效: %w", err)
	}
	if sbom == nil {
		return errors.New("SBOM 必须为对象")
	}
	var compatibility map[string]any
	if err := json.Unmarshal([]byte(version.CompatibilityJSON), &compatibility); err != nil {
		return fmt.Errorf("兼容性声明无效: %w", err)
	}
	if compatibility == nil {
		return errors.New("兼容性声明必须为对象")
	}
	return nil
}

func firstNonEmpty(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
