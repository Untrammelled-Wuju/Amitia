package dataportability

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/argon2"
	amitialog "log"
)

const (
	encryptedFileMagic = "AMITIAE1"
	encryptedVersion   = 1
	encryptionChunk    = 1 << 20
	encryptionSaltSize = 16
	encryptionKeySize  = 32
	encryptionNonce    = 12
)

type encryptedFileHeader struct {
	Version    int    `json:"version"`
	Algorithm  string `json:"algorithm"`
	KDF        string `json:"kdf"`
	Salt       []byte `json:"salt"`
	Nonce      []byte `json:"nonce"`
	MemoryKiB  uint32 `json:"memoryKiB"`
	Iterations uint32 `json:"iterations"`
	Threads    uint8  `json:"threads"`
	ChunkBytes int    `json:"chunkBytes"`
}

func IsEncryptedFile(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	magic := make([]byte, len(encryptedFileMagic))
	if _, err := io.ReadFull(file, magic); err != nil {
		return false
	}
	return string(magic) == encryptedFileMagic
}

func EncryptFile(sourcePath, destinationPath, passphrase string) error {
	if passphrase == "" {
		return ErrBackupPassphraseRequired
	}
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o700); err != nil {
		return err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	keepDestination := false
	defer func() {
		if err := destination.Close(); err != nil {
			amitialog.Printf("[original-error] %v", err)
		}
		if !keepDestination {
			if err := os.Remove(destinationPath); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
		}
	}()

	salt := make([]byte, encryptionSaltSize)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	noncePrefix := make([]byte, 8)
	if _, err := rand.Read(noncePrefix); err != nil {
		return err
	}
	header := encryptedFileHeader{
		Version:    encryptedVersion,
		Algorithm:  "AES-256-GCM",
		KDF:        "argon2id",
		Salt:       salt,
		Nonce:      noncePrefix,
		MemoryKiB:  64 * 1024,
		Iterations: 3,
		Threads:    2,
		ChunkBytes: encryptionChunk,
	}
	headerData, err := json.Marshal(header)
	if err != nil {
		return err
	}
	if _, err := destination.Write([]byte(encryptedFileMagic)); err != nil {
		return err
	}
	if err := binary.Write(destination, binary.BigEndian, uint32(len(headerData))); err != nil {
		return err
	}
	if _, err := destination.Write(headerData); err != nil {
		return err
	}

	key := argon2.IDKey([]byte(passphrase), salt, header.Iterations, header.MemoryKiB, header.Threads, encryptionKeySize)
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	reader := bufio.NewReaderSize(source, encryptionChunk)
	buffer := make([]byte, encryptionChunk)
	var counter uint32
	writeChunk := func(plaintext []byte) error {
		nonce := chunkNonce(noncePrefix, counter)
		ciphertext := gcm.Seal(nil, nonce, plaintext, headerData)
		if err := binary.Write(destination, binary.BigEndian, uint32(len(plaintext))); err != nil {
			return err
		}
		if err := binary.Write(destination, binary.BigEndian, uint32(len(ciphertext))); err != nil {
			return err
		}
		if _, err := destination.Write(ciphertext); err != nil {
			return err
		}
		counter++
		return nil
	}
	for {
		n, readErr := io.ReadFull(reader, buffer)
		if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
			return readErr
		}
		if err := writeChunk(buffer[:n]); err != nil {
			return err
		}
		if n == 0 || errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF) {
			if n > 0 {
				if err := writeChunk(nil); err != nil {
					return err
				}
			}
			break
		}
	}
	if err := destination.Sync(); err != nil {
		return err
	}
	keepDestination = true
	return nil
}

func DecryptFile(sourcePath, destinationPath, passphrase string) error {
	if passphrase == "" {
		return ErrBackupPassphraseRequired
	}
	if err := os.MkdirAll(filepath.Dir(destinationPath), 0o700); err != nil {
		return err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	magic := make([]byte, len(encryptedFileMagic))
	if _, err := io.ReadFull(source, magic); err != nil || string(magic) != encryptedFileMagic {
		return ErrRestoreDecryptFailed
	}
	var headerLength uint32
	if err := binary.Read(source, binary.BigEndian, &headerLength); err != nil || headerLength == 0 || headerLength > 1<<20 {
		return ErrRestoreDecryptFailed
	}
	headerData := make([]byte, headerLength)
	if _, err := io.ReadFull(source, headerData); err != nil {
		return ErrRestoreDecryptFailed
	}
	var header encryptedFileHeader
	if err := json.Unmarshal(headerData, &header); err != nil {
		return ErrRestoreDecryptFailed
	}
	if header.Version != encryptedVersion ||
		header.Algorithm != "AES-256-GCM" ||
		header.KDF != "argon2id" ||
		len(header.Salt) != encryptionSaltSize ||
		len(header.Nonce) != 8 ||
		header.ChunkBytes <= 0 ||
		header.ChunkBytes > 16<<20 {
		return ErrRestoreDecryptFailed
	}

	destination, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	keepDestination := false
	defer func() {
		if !keepDestination {
			if err := os.Remove(destinationPath); err != nil {
				amitialog.Printf("[original-error] %v", err)
			}
		}
	}()
	closeWithError := func(cause error) error {
		closeErr := destination.Close()
		if cause != nil {
			return cause
		}
		return closeErr
	}

	key := argon2.IDKey([]byte(passphrase), header.Salt, header.Iterations, header.MemoryKiB, header.Threads, encryptionKeySize)
	block, err := aes.NewCipher(key)
	if err != nil {
		return closeWithError(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return closeWithError(err)
	}
	var counter uint32
	for {
		var plainLength, cipherLength uint32
		if err := binary.Read(source, binary.BigEndian, &plainLength); err != nil {
			return closeWithError(ErrRestoreDecryptFailed)
		}
		if err := binary.Read(source, binary.BigEndian, &cipherLength); err != nil {
			return closeWithError(ErrRestoreDecryptFailed)
		}
		if plainLength > uint32(header.ChunkBytes) || cipherLength != plainLength+uint32(gcm.Overhead()) || cipherLength > uint32(header.ChunkBytes)+uint32(gcm.Overhead()) {
			return closeWithError(ErrRestoreDecryptFailed)
		}
		ciphertext := make([]byte, cipherLength)
		if _, err := io.ReadFull(source, ciphertext); err != nil {
			return closeWithError(ErrRestoreDecryptFailed)
		}
		plaintext, err := gcm.Open(nil, chunkNonce(header.Nonce, counter), ciphertext, headerData)
		if err != nil {
			return closeWithError(ErrRestoreDecryptFailed)
		}
		if len(plaintext) > 0 {
			if _, err := destination.Write(plaintext); err != nil {
				return closeWithError(err)
			}
		}
		counter++
		if plainLength == 0 {
			break
		}
	}
	if err := destination.Sync(); err != nil {
		return closeWithError(err)
	}
	if err := closeWithError(nil); err != nil {
		return err
	}
	keepDestination = true
	return nil
}

func chunkNonce(prefix []byte, counter uint32) []byte {
	nonce := make([]byte, encryptionNonce)
	copy(nonce, prefix)
	binary.BigEndian.PutUint32(nonce[8:], counter)
	return nonce
}
