package dataportability

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

const maxManifestSizeBytes = 1 << 20

type VerificationResult struct {
	Format             string   `json:"format"`
	FormatVersion      int      `json:"formatVersion"`
	PackageIntegrity   bool     `json:"packageIntegrity"`
	ComponentsVerified int      `json:"componentsVerified"`
	ComponentsFailed   int      `json:"componentsFailed"`
	Errors             []string `json:"errors,omitempty"`
}

func VerifyArchive(archivePath string) (*VerificationResult, error) {
	result := &VerificationResult{}

	f, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	if info.Size() > MaxPackageSizeBytes {
		result.Errors = append(result.Errors, "package exceeds maximum size")
		return result, ErrBackupCompressionInvalid
	}

	zr, err := zip.NewReader(f, info.Size())
	if err != nil {
		return nil, fmt.Errorf("%w: 无法解析压缩包: %v", ErrImportPackageInvalid, err)
	}
	if len(zr.File) == 0 || len(zr.File) > MaxArchiveEntries {
		return result, fmt.Errorf("%w: 归档条目数无效(%d)", ErrImportPackageInvalid, len(zr.File))
	}
	seenEntries := make(map[string]bool, len(zr.File))
	var archiveUncompressed uint64
	for _, entry := range zr.File {
		if seenEntries[entry.Name] {
			return result, fmt.Errorf("%w: 归档条目重复(%s)", ErrImportPackageInvalid, entry.Name)
		}
		seenEntries[entry.Name] = true
		if entry.UncompressedSize64 > MaxSingleResource {
			return result, ErrBackupCompressionInvalid
		}
		archiveUncompressed += entry.UncompressedSize64
		if archiveUncompressed > MaxTotalResources {
			return result, ErrBackupCompressionInvalid
		}
	}

	manifestFile := findFile(zr, "manifest.json")
	if manifestFile == nil {
		result.Errors = append(result.Errors, ErrBackupManifestInvalid.Error())
		return result, ErrBackupManifestInvalid
	}

	mf, err := manifestFile.Open()
	if err != nil {
		return nil, err
	}
	defer mf.Close()

	if manifestFile.UncompressedSize64 > maxManifestSizeBytes {
		result.Errors = append(result.Errors, "manifest exceeds maximum size")
		return result, ErrBackupCompressionInvalid
	}
	manifestData, err := io.ReadAll(mf)
	if err != nil {
		return nil, err
	}

	var manifest BackupManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, ErrBackupManifestInvalid
	}

	result.Format = manifest.Format
	result.FormatVersion = manifest.FormatVersion

	if manifest.Format != FormatName {
		return result, ErrBackupManifestInvalid
	}

	var totalCompressed, totalUncompressed uint64
	seenComponents := make(map[string]bool, len(manifest.Components))
	for _, comp := range manifest.Components {
		if comp.ID == "" || seenComponents[comp.ID] {
			return result, ErrBackupManifestInvalid
		}
		seenComponents[comp.ID] = true
		var matched *zip.File
		for _, zf := range zr.File {
			isFullPath := zf.Name == comp.Path
			isDataset := comp.Kind == string(KindDataset) && zf.Name == "datasets/"+comp.ID+".ndjson"
			isManifest := comp.Kind == string(KindManifest) && zf.Name == "manifest.json"
			if isFullPath || isDataset || isManifest {
				matched = zf
				break
			}
		}

		if matched == nil {
			if comp.Required {
				result.ComponentsFailed++
				result.Errors = append(result.Errors, "missing required component: "+comp.ID)
			}
			continue
		}

		totalCompressed += matched.CompressedSize64
		totalUncompressed += matched.UncompressedSize64
		if totalUncompressed > MaxTotalResources {
			return result, ErrBackupCompressionInvalid
		}

		expected, decodeErr := hex.DecodeString(comp.SHA256)
		if len(comp.SHA256) != sha256.Size*2 || decodeErr != nil {
			result.ComponentsFailed++
			result.Errors = append(result.Errors, "missing or invalid checksum: "+comp.ID)
			continue
		}
		rc, err := matched.Open()
		if err != nil {
			result.ComponentsFailed++
			result.Errors = append(result.Errors, "open component: "+comp.ID+": "+err.Error())
			continue
		}
		h := sha256.New()
		_, copyErr := io.Copy(h, rc)
		closeErr := rc.Close()
		if copyErr != nil || closeErr != nil {
			result.ComponentsFailed++
			result.Errors = append(result.Errors, "read component: "+comp.ID+": "+errors.Join(copyErr, closeErr).Error())
			continue
		}
		if actual := h.Sum(nil); !bytes.Equal(actual, expected) {
			result.ComponentsFailed++
			result.Errors = append(result.Errors, "checksum mismatch: "+comp.ID)
			continue
		}

		result.ComponentsVerified++
	}

	if totalCompressed > 0 {
		ratio := float64(totalUncompressed) / float64(totalCompressed)
		if ratio > MaxCompressionRatio {
			result.Errors = append(result.Errors, "compression ratio exceeds limit")
			return result, ErrBackupCompressionInvalid
		}
	}

	result.PackageIntegrity = result.ComponentsFailed == 0
	return result, nil
}

func findFile(zr *zip.Reader, name string) *zip.File {
	for _, f := range zr.File {
		if f.Name == name {
			return f
		}
	}
	return nil
}
