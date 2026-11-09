package packageverify

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
	"github.com/u-ai/backend/internal/extension/kernel/trust"
)

func VerifyOfficialPublisherPackage(ctx context.Context, packagePath string) error {
	pkg, err := amitiax.OpenArchive(packagePath)
	if err != nil {
		return fmt.Errorf("打开官方插件包: %w", err)
	}
	if err := amitiax.VerifyIntegrity(pkg); err != nil {
		return fmt.Errorf("验证官方插件完整性: %w", err)
	}
	if len(pkg.V2Signature) == 0 {
		return errors.New("官方插件缺少 V2 签名")
	}
	document, err := trust.ParseSignatureDocument(pkg.V2Signature)
	if err != nil {
		return fmt.Errorf("解析官方插件签名: %w", err)
	}
	if document.PublisherID != trust.OfficialPublisherID {
		return fmt.Errorf("官方插件发布者不匹配: %s", document.PublisherID)
	}
	publicKeyHex := strings.TrimSpace(os.Getenv("AMITIA_OFFICIAL_PLUGIN_PUBLIC_KEY"))
	if publicKeyHex == "" {
		publicKeyHex = trust.DefaultOfficialPublisherPublicKey
	}
	publicKeyHex = strings.TrimPrefix(strings.TrimPrefix(publicKeyHex, "0x"), "0X")
	publicKeyBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return fmt.Errorf("解析官方插件信任根: %w", err)
	}
	if len(publicKeyBytes) != ed25519.PublicKeySize {
		return errors.New("官方插件信任根长度无效")
	}
	publicKey := ed25519.PublicKey(publicKeyBytes)
	keyID := trust.ComputeKeyID(publicKey)
	if document.KeyID != keyID {
		return fmt.Errorf("官方插件 Key ID 不匹配: %s", document.KeyID)
	}
	store := trust.NewPublisherStore()
	identity := trust.PublisherIdentity{
		PublisherID: trust.OfficialPublisherID,
		DisplayName: "Amitia",
		Keys: []trust.PublisherKey{{
			KeyID: keyID, PublisherID: trust.OfficialPublisherID, PublicKey: publicKey,
			Algorithm: trust.AlgorithmEd25519, State: trust.KeyStateActive, CreatedAt: time.Now().UTC(),
		}},
		TrustLevel: trust.TrustLevelOfficial,
		Source:     trust.TrustSourceBuiltin,
	}
	if err := store.RegisterBuiltinRoot(trust.OfficialPublisherID, identity); err != nil {
		return fmt.Errorf("注册官方插件信任根: %w", err)
	}
	manifestHash := ""
	if entry, ok := pkg.Integrity.Files[amitiax.ManifestFile]; ok {
		manifestHash = entry.Hash
		if !strings.HasPrefix(manifestHash, "sha256:") {
			manifestHash = "sha256:" + manifestHash
		}
	}
	artifactEntries := make([]trust.ArtifactEntry, 0, len(pkg.Integrity.Files))
	for filePath, entry := range pkg.Integrity.Files {
		hash := entry.Hash
		if !strings.HasPrefix(hash, "sha256:") {
			hash = "sha256:" + hash
		}
		artifactEntries = append(artifactEntries, trust.ArtifactEntry{Path: filePath, Size: entry.Size, SHA256: hash})
	}
	result := trust.NewSignatureVerifier(store).VerifyPackage(ctx, trust.PackageVerificationInput{
		Document: document, ActualExtensionID: pkg.Manifest.Plugin.ID,
		ActualVersion: pkg.Manifest.Plugin.Version, ActualManifestVersion: pkg.Manifest.SchemaVersion,
		ActualManifestHash: manifestHash, ActualContentTreeHash: pkg.Tree.TreeHash,
		ActualArtifactHash: trust.ComputeCanonicalArtifactHash(artifactEntries),
	})
	if !trust.IsSignatureValid(result) {
		return fmt.Errorf("官方插件签名无效: %s: %s", result.Status, result.Reason)
	}
	return nil
}
