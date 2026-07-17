package extension

import (
	"archive/zip"
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

type parsedExtensionPackage struct {
	Format       PackageFormat
	Source       string
	Files        map[string][]byte
	Raw          []byte
	PackageHash  string
	Manifest     Manifest
	ManifestRaw  json.RawMessage
	Signature    PackageSignatureView
	AgentSkill   *parsedAgentSkill
	Workflow     *WorkflowDefinition
	WorkflowRaw  json.RawMessage
	Schemas      map[string]json.RawMessage
	Tests        json.RawMessage
	Warnings     []string
	FileViews    []PackageFileView
	SignedDigest string
}

type packageSignatureDocument struct {
	Algorithm    string `json:"algorithm"`
	KeyID        string `json:"keyId"`
	PublicKey    string `json:"publicKey"`
	Signature    string `json:"signature"`
	SignedDigest string `json:"signedDigest"`
	DisplayName  string `json:"displayName,omitempty"`
}

var packageDrivePattern = regexp.MustCompile(`^[A-Za-z]:`)

var packageForbiddenExtensions = map[string]bool{
	".exe": true, ".com": true, ".bat": true, ".cmd": true, ".msi": true, ".dll": true,
	".sys": true, ".scr": true, ".pif": true, ".cpl": true, ".so": true, ".dylib": true,
	".app": true, ".wasm": true, ".class": true, ".jar": true, ".apk": true, ".deb": true, ".rpm": true,
}

var packageNestedArchives = map[string]bool{
	".zip": true, ".rar": true, ".7z": true, ".tar": true, ".gz": true, ".tgz": true, ".bz2": true, ".xz": true,
}

func readPackageZIP(raw []byte, limits PackageLimits) (map[string][]byte, []PackageFileView, error) {
	if len(raw) < 4 || !bytes.Equal(raw[:2], []byte{'P', 'K'}) {
		return nil, nil, NewExtensionError(ErrPackageInvalidArchive, "扩展包不是有效 ZIP", "", false, nil)
	}
	if int64(len(raw)) > limits.MaxExpandedBytes {
		return nil, nil, NewExtensionError(ErrPackageArchiveLimit, "扩展包超过上传限制", "", false, nil)
	}
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, nil, NewExtensionError(ErrPackageInvalidArchive, "扩展包无法读取", "", false, err)
	}
	if len(reader.File) > limits.MaxFiles {
		return nil, nil, NewExtensionError(ErrPackageArchiveLimit, "扩展包文件数量超过限制", "", false, nil)
	}
	files := map[string][]byte{}
	canonical := map[string]string{}
	views := make([]PackageFileView, 0, len(reader.File))
	var total int64
	for _, item := range reader.File {
		if item.NonUTF8 {
			return nil, nil, NewExtensionError(ErrPackageInvalidArchive, "扩展包文件名不是 UTF-8", "", false, nil)
		}
		name, pathErr := validatePackagePath(item.Name, limits)
		if pathErr != nil {
			return nil, nil, pathErr
		}
		if strings.HasSuffix(item.Name, "/") {
			continue
		}
		mode := item.Mode()
		if mode&os.ModeType != 0 || mode.IsDir() {
			return nil, nil, NewExtensionError(ErrPackageInvalidArchive, "扩展包包含链接或特殊文件", name, false, nil)
		}
		key := strings.ToLower(norm.NFC.String(name))
		if previous, exists := canonical[key]; exists {
			return nil, nil, NewExtensionError(ErrPackageInvalidArchive, "扩展包包含冲突路径", previous+" / "+name, false, nil)
		}
		canonical[key] = name
		if item.UncompressedSize64 > uint64(limits.MaxFileBytes) {
			return nil, nil, NewExtensionError(ErrPackageArchiveLimit, "扩展包单文件超过限制", name, false, nil)
		}
		if item.CompressedSize64 == 0 && item.UncompressedSize64 > 0 || item.CompressedSize64 > 0 && item.UncompressedSize64/item.CompressedSize64 > limits.MaxCompressionRatio {
			return nil, nil, NewExtensionError(ErrPackageArchiveLimit, "扩展包压缩比超过限制", name, false, nil)
		}
		total += int64(item.UncompressedSize64)
		if total > limits.MaxExpandedBytes {
			return nil, nil, NewExtensionError(ErrPackageArchiveLimit, "扩展包展开大小超过限制", "", false, nil)
		}
		rc, openErr := item.Open()
		if openErr != nil {
			return nil, nil, NewExtensionError(ErrPackageInvalidArchive, "扩展包文件无法读取", name, false, openErr)
		}
		content, readErr := io.ReadAll(io.LimitReader(rc, limits.MaxFileBytes+1))
		closeErr := rc.Close()
		if readErr != nil || closeErr != nil {
			return nil, nil, NewExtensionError(ErrPackageInvalidArchive, "扩展包文件读取失败", name, false, readErr)
		}
		if err := validatePackageFile(name, content); err != nil {
			return nil, nil, err
		}
		files[name] = content
		views = append(views, PackageFileView{Path: name, Size: int64(len(content)), Kind: packageFileKind(name)})
	}
	sort.Slice(views, func(i, j int) bool { return views[i].Path < views[j].Path })
	return files, views, nil
}

func validatePackagePath(input string, limits PackageLimits) (string, error) {
	if !utf8.ValidString(input) || strings.ContainsRune(input, 0) {
		return "", NewExtensionError(ErrPackagePathTraversal, "扩展包路径编码无效", "", false, nil)
	}
	if strings.HasPrefix(input, "/") || strings.HasPrefix(input, `\`) || strings.HasPrefix(input, "//") || packageDrivePattern.MatchString(input) {
		return "", NewExtensionError(ErrPackagePathTraversal, "扩展包包含绝对路径", "", false, nil)
	}
	normalized := strings.ReplaceAll(input, `\`, "/")
	clean := path.Clean(normalized)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || clean != normalized {
		return "", NewExtensionError(ErrPackagePathTraversal, "扩展包路径越界或未规范化", input, false, nil)
	}
	parts := strings.Split(clean, "/")
	if len(parts) > limits.MaxDepth {
		return "", NewExtensionError(ErrPackageArchiveLimit, "扩展包目录层级超过限制", clean, false, nil)
	}
	for _, part := range parts {
		base := strings.TrimSuffix(strings.ToLower(part), path.Ext(strings.ToLower(part)))
		if strings.Trim(part, " .") != part || windowsReservedName(base) {
			return "", NewExtensionError(ErrPackageInvalidArchive, "扩展包路径包含 Windows 保留名称", clean, false, nil)
		}
	}
	return clean, nil
}

func validatePackageFile(name string, content []byte) error {
	ext := strings.ToLower(path.Ext(name))
	if packageForbiddenExtensions[ext] {
		return NewExtensionError(ErrPackageEntryUnsupported, "扩展包包含禁止文件类型", name, false, nil)
	}
	if packageNestedArchives[ext] {
		return NewExtensionError(ErrPackageEntryUnsupported, "扩展包不允许嵌套归档", name, false, nil)
	}
	if len(content) >= 2 && bytes.Equal(content[:2], []byte{'M', 'Z'}) || len(content) >= 4 && (bytes.Equal(content[:4], []byte{0x7f, 'E', 'L', 'F'}) || bytes.Equal(content[:4], []byte{0, 'a', 's', 'm'})) {
		return NewExtensionError(ErrPackageEntryUnsupported, "扩展包包含可执行二进制", name, false, nil)
	}
	mime := http.DetectContentType(content)
	if strings.HasPrefix(mime, "text/") || strings.Contains(mime, "json") || strings.Contains(mime, "xml") || mime == "application/octet-stream" && utf8.Valid(content) || strings.HasPrefix(mime, "image/") || strings.HasPrefix(mime, "audio/") || strings.HasPrefix(mime, "video/") || len(content) == 0 {
		return nil
	}
	return NewExtensionError(ErrPackageEntryUnsupported, "扩展包包含未知二进制", name, false, nil)
}

func packageFileKind(name string) string {
	switch {
	case name == "manifest.json":
		return "manifest"
	case strings.HasPrefix(name, "schemas/"):
		return "schema"
	case strings.HasPrefix(name, "workflows/"):
		return "workflow"
	case strings.Contains(name, "scripts/"):
		return "script-disabled"
	case strings.Contains(name, "assets/"):
		return "asset"
	case strings.Contains(name, "references/"):
		return "reference"
	default:
		return "file"
	}
}

func stablePackageZIP(files map[string][]byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	keys := make([]string, 0, len(files))
	for key := range files {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		header := &zip.FileHeader{Name: key, Method: zip.Deflate}
		header.SetMode(0o444)
		header.SetModTime(time.Unix(0, 0).UTC())
		entry, err := writer.CreateHeader(header)
		if err != nil {
			return nil, err
		}
		if _, err := entry.Write(files[key]); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func buildChecksums(files map[string][]byte) []byte {
	keys := make([]string, 0, len(files))
	for key := range files {
		if key != "checksums.sha256" && key != "signature.json" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var builder strings.Builder
	for _, key := range keys {
		hash := sha256.Sum256(files[key])
		builder.WriteString(hex.EncodeToString(hash[:]))
		builder.WriteString("  ")
		builder.WriteString(key)
		builder.WriteByte('\n')
	}
	return []byte(builder.String())
}

func validateChecksums(files map[string][]byte) error {
	raw, ok := files["checksums.sha256"]
	if !ok {
		return NewExtensionError(ErrPackageChecksumMissing, "扩展包缺少 checksums.sha256", "", false, nil)
	}
	listed := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		parts := strings.SplitN(strings.TrimSuffix(line, "\r"), "  ", 2)
		if len(parts) != 2 || len(parts[0]) != 64 {
			return NewExtensionError(ErrPackageChecksumInvalid, "Checksum 格式无效", "", false, nil)
		}
		name, err := validatePackagePath(parts[1], DefaultPackageLimits())
		if err != nil || name == "checksums.sha256" || name == "signature.json" {
			return NewExtensionError(ErrPackageChecksumInvalid, "Checksum 路径无效", parts[1], false, err)
		}
		if _, exists := listed[name]; exists {
			return NewExtensionError(ErrPackageChecksumInvalid, "Checksum 包含重复路径", name, false, nil)
		}
		listed[name] = strings.ToLower(parts[0])
	}
	for name, content := range files {
		if name == "checksums.sha256" || name == "signature.json" {
			continue
		}
		expected, exists := listed[name]
		if !exists {
			return NewExtensionError(ErrPackageUnlistedFile, "扩展包包含未列入 Checksum 的文件", name, false, nil)
		}
		hash := sha256.Sum256(content)
		if hex.EncodeToString(hash[:]) != expected {
			return NewExtensionError(ErrPackageChecksumMismatch, "扩展包文件 Checksum 不匹配", name, false, nil)
		}
		delete(listed, name)
	}
	if len(listed) > 0 {
		keys := make([]string, 0, len(listed))
		for key := range listed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return NewExtensionError(ErrPackageMissingFile, "Checksum 引用了缺失文件", strings.Join(keys, ", "), false, nil)
	}
	return nil
}

func packageCanonicalDigest(files map[string][]byte) string {
	keys := make([]string, 0, len(files))
	for key := range files {
		if key != "signature.json" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, key := range keys {
		fileHash := sha256.Sum256(files[key])
		h.Write([]byte(key))
		h.Write([]byte{0})
		h.Write(fileHash[:])
	}
	return hex.EncodeToString(h.Sum(nil))
}

func verifyPackageSignature(files map[string][]byte, trusted bool) (PackageSignatureView, string, error) {
	digest := packageCanonicalDigest(files)
	raw, ok := files["signature.json"]
	if !ok {
		return PackageSignatureView{Status: PackageSignatureUnsigned}, digest, nil
	}
	var document packageSignatureDocument
	if json.Unmarshal(raw, &document) != nil || document.Algorithm != "ed25519" {
		return PackageSignatureView{Status: PackageSignatureInvalid}, digest, NewExtensionError(ErrPackageSignatureInvalid, "扩展包签名格式或算法无效", "", false, nil)
	}
	publicKey, keyErr := base64.StdEncoding.DecodeString(document.PublicKey)
	signature, signatureErr := base64.StdEncoding.DecodeString(document.Signature)
	expected := "sha256:" + digest
	fingerprintHash := sha256.Sum256(publicKey)
	fingerprint := "sha256:" + hex.EncodeToString(fingerprintHash[:])
	if keyErr != nil || signatureErr != nil || len(publicKey) != ed25519.PublicKeySize || document.SignedDigest != expected || document.KeyID != fingerprint || !ed25519.Verify(ed25519.PublicKey(publicKey), []byte(expected), signature) {
		return PackageSignatureView{Status: PackageSignatureInvalid, Fingerprint: fingerprint, Algorithm: document.Algorithm}, digest, NewExtensionError(ErrPackageSignatureInvalid, "扩展包签名验证失败", "", false, nil)
	}
	status := PackageSignatureUntrusted
	if trusted {
		status = PackageSignatureTrusted
	}
	return PackageSignatureView{Status: status, Fingerprint: fingerprint, Algorithm: document.Algorithm, DisplayName: document.DisplayName}, digest, nil
}

func packageHash(raw []byte) string {
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}
