package dataportability

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
)

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
		return nil, ErrImportPackageInvalid
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
	for _, comp := range manifest.Components {
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

		if comp.SHA256 != "" {
			rc, err := matched.Open()
			if err != nil {
				result.ComponentsFailed++
				continue
			}
			h := sha256.New()
			io.Copy(h, rc)
			rc.Close()
			actualSum := hex.EncodeToString(h.Sum(nil))
			if actualSum != comp.SHA256 {
				result.ComponentsFailed++
				result.Errors = append(result.Errors, "checksum mismatch: "+comp.ID)
				continue
			}
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
