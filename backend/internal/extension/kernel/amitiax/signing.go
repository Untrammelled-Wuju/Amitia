package amitiax

import (
	"archive/zip"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

func signatureMessage(publisherID, treeHash string) string {
	return publisherID + ":" + treeHash
}

func SignPackage(pkg *Package, privateKey ed25519.PrivateKey, keyID string, publisherID string) (*SignatureDoc, error) {
	if pkg == nil {
		return nil, fmt.Errorf("amitiax: package is nil")
	}
	treeHash := pkg.Tree.TreeHash
	if treeHash == "" {
		return nil, fmt.Errorf("amitiax: content tree hash empty")
	}
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("amitiax: invalid private key size")
	}
	msg := signatureMessage(publisherID, treeHash)
	sig := ed25519.Sign(privateKey, []byte(msg))
	return &SignatureDoc{
		Algorithm:   "ed25519",
		KeyID:       keyID,
		Signature:   sig,
		SignedAt:    time.Now().UTC(),
		PublisherID: publisherID,
	}, nil
}

func WriteSignatureToArchive(archivePath string, sig *SignatureDoc) error {
	if sig == nil {
		return fmt.Errorf("amitiax: signature is nil")
	}
	data, err := json.MarshalIndent(sig, "", "  ")
	if err != nil {
		return fmt.Errorf("amitiax: marshal signature: %w", err)
	}
	tmpPath := archivePath + ".tmp"
	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	w := zip.NewWriter(tmpFile)
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("%w: %v", ErrInvalidArchive, err)
	}
	for _, f := range r.File {
		name := normalizePath(f.Name)
		if name == SignatureFile {
			continue
		}
		out, err := w.CreateHeader(&zip.FileHeader{
			Name:   f.Name,
			Method: f.Method,
		})
		if err != nil {
			r.Close()
			w.Close()
			tmpFile.Close()
			os.Remove(tmpPath)
			return err
		}
		rc, err := f.Open()
		if err != nil {
			r.Close()
			w.Close()
			tmpFile.Close()
			os.Remove(tmpPath)
			return err
		}
		_, copyErr := io.Copy(out, rc)
		rc.Close()
		if copyErr != nil {
			r.Close()
			w.Close()
			tmpFile.Close()
			os.Remove(tmpPath)
			return copyErr
		}
	}
	r.Close()
	sigWriter, err := w.Create(SignatureFile)
	if err != nil {
		w.Close()
		tmpFile.Close()
		os.Remove(tmpPath)
		return err
	}
	if _, err := sigWriter.Write(data); err != nil {
		w.Close()
		tmpFile.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := w.Close(); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, archivePath); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}
