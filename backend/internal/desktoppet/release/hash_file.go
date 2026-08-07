// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package release

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

func HashFile(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	hash := sha256.New()
	written, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, err
	}

	return hex.EncodeToString(hash.Sum(nil)), written, nil
}
