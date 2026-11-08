package dataportability

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestEncryptFileRoundTripsChunkBoundaries(t *testing.T) {
	for _, size := range []int{0, 1, encryptionChunk - 1, encryptionChunk, encryptionChunk*2 + 17} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			dir := t.TempDir()
			plain := bytes.Repeat([]byte{0x5a}, size)
			source := filepath.Join(dir, "plain")
			encrypted := filepath.Join(dir, "encrypted")
			output := filepath.Join(dir, "output")
			if err := os.WriteFile(source, plain, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := EncryptFile(source, encrypted, "chunk boundary passphrase"); err != nil {
				t.Fatal(err)
			}
			if err := DecryptFile(encrypted, output, "chunk boundary passphrase"); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, plain) {
				t.Fatalf("round trip mismatch: got %d bytes, want %d", len(got), len(plain))
			}
		})
	}
}

func TestDecryptFileRejectsMalformedBackupsAndRetainsCause(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "plain")
	encrypted := filepath.Join(dir, "encrypted")
	output := filepath.Join(dir, "output")
	content := []byte("QA backup payload")
	if err := os.WriteFile(plain, content, 0o600); err != nil {
		t.Fatal(err)
	}
	const password = "isolated QA passphrase"
	if err := EncryptFile(plain, encrypted, password); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	const headerStart = len(encryptedFileMagic) + 4
	headerEnd := headerStart + int(binary.BigEndian.Uint32(data[len(encryptedFileMagic):headerStart]))
	var header encryptedFileHeader
	if err := json.Unmarshal(data[headerStart:headerEnd], &header); err != nil {
		t.Fatal(err)
	}
	header.Iterations = 0
	badHeader, err := json.Marshal(header)
	if err != nil {
		t.Fatal(err)
	}
	malformed := binary.BigEndian.AppendUint32([]byte(encryptedFileMagic), uint32(len(badHeader)))
	malformed = append(malformed, badHeader...)
	malformed = append(malformed, data[headerEnd:]...)
	for _, tc := range []struct {
		name, pass, detail string
		data               []byte
		cause              error
	}{
		{"wrong password", "incorrect", "message authentication failed", data, nil},
		{"truncated", password, "unexpected EOF", data[:len(data)-8], io.ErrUnexpectedEOF},
		{"trailing", password, "trailing data", append(bytes.Clone(data), 0xff), nil},
		{"zero iterations", password, "iterations=0", malformed, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(encrypted, tc.data, 0o600); err != nil {
				t.Fatal(err)
			}
			err := DecryptFile(encrypted, output, tc.pass)
			if !errors.Is(err, ErrRestoreDecryptFailed) || !strings.Contains(err.Error(), tc.detail) || tc.cause != nil && !errors.Is(err, tc.cause) {
				t.Fatalf("unexpected decryption error: %v", err)
			}
			if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("failed decryption left a destination: %v", err)
			}
		})
	}
	if err := os.WriteFile(encrypted, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := DecryptFile(encrypted, output, password); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(output)
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("round trip: %q, %v", got, err)
	}
}
