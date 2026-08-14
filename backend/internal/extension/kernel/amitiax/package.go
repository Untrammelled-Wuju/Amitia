package amitiax

import (
	"archive/zip"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/manifest_v2"
)

const (
	ManifestFile         = "manifest.json"
	IntegrityDir         = "integrity/"
	IntegrityFiles       = "integrity/files.json"
	IntegrityTree        = "integrity/content-tree.json"
	ModulesDir           = "modules/"
	ResourcesDir         = "resources/"
	AssetsDir            = "assets/"
	MigrationsDir        = "migrations/"
	LicensesDir          = "licenses/"
	DocsDir              = "docs/"
	SignaturesDir        = "signatures/"
	SignatureFile        = "signatures/signature.json"
	V2SignatureFile      = "META-INF/amitia-signature.json"
	maxPackageEntryBytes = int64(100 << 20)
)

var allowedRootDirs = map[string]bool{
	"manifest.json": true,
	"integrity":     true,
	"modules":       true,
	"resources":     true,
	"assets":        true,
	"migrations":    true,
	"licenses":      true,
	"docs":          true,
	"signatures":    true,
	"META-INF":      true,
}

var CanonicalIntegrityExcludedPaths = map[string]bool{
	IntegrityFiles:  true,
	IntegrityTree:   true,
	SignatureFile:   true,
	V2SignatureFile: true,
}

func IsCanonicalIntegrityExcludedPath(filePath string) bool {
	return CanonicalIntegrityExcludedPaths[normalizePath(filePath)]
}

type PackageLayout struct {
	ManifestPath    string
	Modules         map[string]string
	Resources       []string
	Assets          []string
	Migrations      []string
	Licenses        []string
	Docs            []string
	Signatures      []string
	IntegrityFiles  string
	IntegrityTree   string
	SignatureFile   string
	V2SignatureFile string
}

type FileEntry struct {
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	Hash     string    `json:"hash"`
	Modified time.Time `json:"modified"`
	IsDir    bool      `json:"isDir,omitempty"`
}

type IntegrityFilesDoc struct {
	Algorithm   string               `json:"algorithm"`
	Files       map[string]FileEntry `json:"files"`
	GeneratedAt time.Time            `json:"generatedAt"`
}

type IntegrityTreeDoc struct {
	Algorithm   string    `json:"algorithm"`
	TreeHash    string    `json:"treeHash"`
	GeneratedAt time.Time `json:"generatedAt"`
}

type SignatureDoc struct {
	Algorithm   string    `json:"algorithm"`
	KeyID       string    `json:"keyId"`
	Signature   []byte    `json:"signature"`
	SignedAt    time.Time `json:"signedAt"`
	PublisherID string    `json:"publisherId,omitempty"`
}

type Package struct {
	Manifest    manifest_v2.Manifest
	Layout      PackageLayout
	Files       []FileEntry
	Integrity   IntegrityFilesDoc
	Tree        IntegrityTreeDoc
	Signatures  *SignatureDoc
	V2Signature json.RawMessage
}

var (
	ErrInvalidArchive          = errors.New("amitiax: invalid archive")
	ErrManifestMissing         = errors.New("amitiax: manifest.json missing")
	ErrInvalidStructure        = errors.New("amitiax: invalid package structure")
	ErrPathTraversal           = errors.New("amitiax: path traversal detected")
	ErrIntegrityMissing        = errors.New("amitiax: integrity files missing")
	ErrIntegrityMismatch       = errors.New("amitiax: integrity mismatch")
	ErrSignatureMissing        = errors.New("amitiax: signature missing")
	ErrSignatureInvalid        = errors.New("amitiax: invalid signature")
	ErrUnsupportedSigAlgorithm = errors.New("amitiax: unsupported signature algorithm")
)

func OpenArchive(archivePath string) (*Package, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidArchive, err)
	}
	defer reader.Close()
	return parsePackage(&reader.Reader)
}

func parsePackage(reader *zip.Reader) (*Package, error) {
	pkg := &Package{}
	var manifestData []byte
	var files []FileEntry
	layout := PackageLayout{
		Modules: make(map[string]string),
	}
	for _, f := range reader.File {
		if err := validateArchivePath(f.Name); err != nil {
			return nil, err
		}
		name := normalizePath(f.Name)
		if err := validatePath(name); err != nil {
			return nil, err
		}
		entry := FileEntry{
			Path:  name,
			Size:  int64(f.UncompressedSize64),
			IsDir: f.FileInfo().IsDir(),
		}
		if !entry.IsDir {
			data, err := readZipFile(f)
			if err != nil {
				return nil, err
			}
			sum := sha256.Sum256(data)
			entry.Hash = hex.EncodeToString(sum[:])
			switch {
			case name == ManifestFile:
				manifestData = data
				layout.ManifestPath = name
			case name == IntegrityFiles:
				layout.IntegrityFiles = name
				if err := json.Unmarshal(data, &pkg.Integrity); err != nil {
					return nil, fmt.Errorf("%w: parse integrity: %v", ErrInvalidStructure, err)
				}
			case name == IntegrityTree:
				layout.IntegrityTree = name
				if err := json.Unmarshal(data, &pkg.Tree); err != nil {
					return nil, fmt.Errorf("%w: parse tree: %v", ErrInvalidStructure, err)
				}
			case name == SignatureFile:
				layout.SignatureFile = name
				sig := &SignatureDoc{}
				if err := json.Unmarshal(data, sig); err != nil {
					return nil, fmt.Errorf("%w: parse signature: %v", ErrInvalidStructure, err)
				}
				pkg.Signatures = sig
			case name == V2SignatureFile:
				layout.V2SignatureFile = name
				pkg.V2Signature = make(json.RawMessage, len(data))
				copy(pkg.V2Signature, data)
			}
		}
		files = append(files, entry)
		categorizeEntry(name, &layout)
	}
	if len(manifestData) == 0 {
		return nil, ErrManifestMissing
	}
	m, report, err := manifest_v2.ParseValidated(manifestData)
	if err != nil {
		return nil, err
	}
	if report.HasErrors() {
		return nil, fmt.Errorf("%w: manifest validation failed", manifest_v2.ErrInvalidManifest)
	}
	if m.Integrity.ContentTreeHash == "" {
		m.Integrity.ContentTreeHash = pkg.Tree.TreeHash
	} else if pkg.Tree.TreeHash != "" && m.Integrity.ContentTreeHash != pkg.Tree.TreeHash {
		return nil, fmt.Errorf("%w: manifest content tree hash mismatch", ErrIntegrityMismatch)
	}
	pkg.Manifest = m
	pkg.Layout = layout
	pkg.Files = files
	if err := validateStructure(pkg); err != nil {
		return nil, err
	}
	return pkg, nil
}

func categorizeEntry(name string, layout *PackageLayout) {
	parts := strings.SplitN(name, "/", 2)
	if len(parts) < 2 {
		return
	}
	root := parts[0]
	rest := parts[1]
	switch root {
	case "modules":
		modParts := strings.SplitN(rest, "/", 2)
		if len(modParts) >= 1 && modParts[0] != "" {
			modID := modParts[0]
			if _, ok := layout.Modules[modID]; !ok {
				layout.Modules[modID] = filepath.Join(ModulesDir, modID)
			}
		}
	case "resources":
		layout.Resources = append(layout.Resources, name)
	case "assets":
		layout.Assets = append(layout.Assets, name)
	case "migrations":
		layout.Migrations = append(layout.Migrations, name)
	case "licenses":
		layout.Licenses = append(layout.Licenses, name)
	case "docs":
		layout.Docs = append(layout.Docs, name)
	case "signatures":
		layout.Signatures = append(layout.Signatures, name)
	}
}

func validateStructure(pkg *Package) error {
	for _, f := range pkg.Files {
		parts := strings.SplitN(f.Path, "/", 2)
		root := parts[0]
		if !allowedRootDirs[root] && root != ManifestFile {
			return fmt.Errorf("%w: unknown root entry %s", ErrInvalidStructure, root)
		}
	}
	if pkg.Layout.ManifestPath == "" {
		return ErrManifestMissing
	}
	for _, mod := range pkg.Manifest.Modules {
		if _, ok := pkg.Layout.Modules[mod.ID]; !ok {
			return fmt.Errorf("%w: module %s missing directory", ErrInvalidStructure, mod.ID)
		}
	}
	if pkg.Layout.IntegrityFiles == "" {
		return fmt.Errorf("%w: integrity/files.json missing", ErrIntegrityMissing)
	}
	if pkg.Layout.IntegrityTree == "" {
		return fmt.Errorf("%w: integrity/content-tree.json missing", ErrIntegrityMissing)
	}
	return nil
}

func normalizePath(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	if !strings.HasPrefix(p, "/") && len(p) > 0 {
		return p
	}
	return strings.TrimPrefix(p, "/")
}

func validatePath(p string) error {
	if strings.Contains(p, "..") {
		return fmt.Errorf("%w: %s", ErrPathTraversal, p)
	}
	if strings.Contains(p, "//") {
		return fmt.Errorf("%w: double slash in %s", ErrInvalidStructure, p)
	}
	return nil
}

func validateArchivePath(raw string) error {
	normalized := strings.ReplaceAll(raw, "\\", "/")
	if normalized == "" || strings.HasPrefix(normalized, "/") || len(normalized) >= 2 && normalized[1] == ':' {
		return fmt.Errorf("%w: absolute path %s", ErrPathTraversal, raw)
	}
	cleaned := path.Clean(normalized)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return fmt.Errorf("%w: %s", ErrPathTraversal, raw)
	}
	return nil
}

func readZipFile(f *zip.File) ([]byte, error) {
	if f.UncompressedSize64 > uint64(maxPackageEntryBytes) {
		return nil, fmt.Errorf("%w: entry exceeds limit", ErrInvalidArchive)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, maxPackageEntryBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxPackageEntryBytes {
		return nil, fmt.Errorf("%w: entry exceeds limit", ErrInvalidArchive)
	}
	return data, nil
}

func ComputeTreeHash(files []FileEntry) string {
	sorted := make([]FileEntry, len(files))
	copy(sorted, files)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Path < sorted[j].Path
	})
	h := sha256.New()
	for _, f := range sorted {
		if f.IsDir {
			continue
		}
		h.Write([]byte(f.Path))
		h.Write([]byte{0})
		h.Write([]byte(f.Hash))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func VerifyIntegrity(pkg *Package) error {
	verifiedFiles := make([]FileEntry, 0, len(pkg.Integrity.Files))
	for _, f := range pkg.Files {
		if f.IsDir {
			continue
		}
		if IsCanonicalIntegrityExcludedPath(f.Path) {
			continue
		}
		entry, ok := pkg.Integrity.Files[f.Path]
		if !ok {
			return fmt.Errorf("%w: %s not in integrity manifest", ErrIntegrityMismatch, f.Path)
		}
		if entry.Hash != f.Hash {
			return fmt.Errorf("%w: %s hash mismatch", ErrIntegrityMismatch, f.Path)
		}
		if entry.Size != f.Size {
			return fmt.Errorf("%w: %s size mismatch", ErrIntegrityMismatch, f.Path)
		}
		verifiedFiles = append(verifiedFiles, f)
	}
	computedTree := ComputeTreeHash(verifiedFiles)
	if pkg.Tree.TreeHash != "" && computedTree != pkg.Tree.TreeHash {
		return fmt.Errorf("%w: tree hash mismatch", ErrIntegrityMismatch)
	}
	return nil
}

func VerifySignature(pkg *Package, publicKey ed25519.PublicKey) error {
	if pkg == nil || pkg.Signatures == nil {
		return ErrSignatureMissing
	}
	if pkg.Signatures.Algorithm != "ed25519" {
		return fmt.Errorf("%w: %s", ErrUnsupportedSigAlgorithm, pkg.Signatures.Algorithm)
	}
	treeHash := pkg.Tree.TreeHash
	if treeHash == "" {
		return fmt.Errorf("%w: content tree hash empty", ErrSignatureInvalid)
	}
	if len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("%w: invalid public key size", ErrSignatureInvalid)
	}
	msg := signatureMessage(pkg.Signatures.PublisherID, treeHash)
	if !ed25519.Verify(publicKey, []byte(msg), pkg.Signatures.Signature) {
		return ErrSignatureInvalid
	}
	return nil
}

func WritePackageToDir(pkg *Package, archivePath, destDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidArchive, err)
	}
	defer reader.Close()
	for _, f := range reader.File {
		name := normalizePath(f.Name)
		if err := validatePath(name); err != nil {
			return err
		}
		dest := filepath.Join(destDir, filepath.FromSlash(name))
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			rc.Close()
			out.Close()
			return err
		}
		rc.Close()
		out.Close()
	}
	return nil
}

func (p *Package) ListModuleFiles(moduleID string) []FileEntry {
	prefix := path.Join(ModulesDir, moduleID) + "/"
	var out []FileEntry
	for _, f := range p.Files {
		if strings.HasPrefix(f.Path, prefix) && !f.IsDir {
			out = append(out, f)
		}
	}
	return out
}
