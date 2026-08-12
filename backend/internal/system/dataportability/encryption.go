package dataportability

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
)

type EncryptionMetadata struct {
	Algorithm  string `json:"algorithm"`
	Nonce      []byte `json:"nonce"`
	Salt       []byte `json:"salt"`
	Iterations int    `json:"iterations"`
}

const (
	keySize   = 32
	nonceSize = 12
	saltSize  = 32
)

func DeriveKey(passphrase string, salt []byte) []byte {
	if salt == nil {
		salt = make([]byte, saltSize)
		rand.Read(salt)
	}
	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(passphrase))
	return h.Sum(nil)
}

func EncryptStream(dst io.Writer, passphrase string) (io.WriteCloser, *EncryptionMetadata, error) {
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, nil, err
	}

	key := DeriveKey(passphrase, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}

	metadata := &EncryptionMetadata{
		Algorithm:  "AES-256-GCM",
		Nonce:      nonce,
		Salt:       salt,
		Iterations: 0,
	}

	writer := &gcmWriter{
		dst:    dst,
		gcm:    gcm,
		nonce:  nonce,
		buf:    make([]byte, 0, 32*1024),
		closed: false,
	}

	return writer, metadata, nil
}

func DecryptStream(src io.Reader, passphrase string, meta *EncryptionMetadata) (io.Reader, error) {
	if meta == nil {
		return nil, ErrBackupPassphraseRequired
	}
	if len(meta.Nonce) != nonceSize {
		return nil, ErrRestoreDecryptFailed
	}
	if len(meta.Salt) != saltSize {
		return nil, ErrRestoreDecryptFailed
	}

	key := DeriveKey(passphrase, meta.Salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return &gcmReader{
		src:   src,
		gcm:   gcm,
		nonce: meta.Nonce,
	}, nil
}

type gcmWriter struct {
	dst    io.Writer
	gcm    cipher.AEAD
	nonce  []byte
	buf    []byte
	closed bool
}

func (w *gcmWriter) Write(p []byte) (int, error) {
	if w.closed {
		return 0, errors.New("writer closed")
	}
	w.buf = append(w.buf, p...)
	return len(p), nil
}

func (w *gcmWriter) Close() error {
	if w.closed {
		return errors.New("already closed")
	}
	w.closed = true

	ct := w.gcm.Seal(nil, w.nonce, w.buf, nil)
	_, err := w.dst.Write(ct)
	return err
}

type gcmReader struct {
	src   io.Reader
	gcm   cipher.AEAD
	nonce []byte
	data  []byte
	eof   bool
}

func (r *gcmReader) Read(p []byte) (int, error) {
	if r.eof && len(r.data) == 0 {
		return 0, io.EOF
	}
	if len(r.data) == 0 && !r.eof {
		ct, err := io.ReadAll(r.src)
		if err != nil && err != io.EOF {
			return 0, err
		}
		if err == io.EOF {
			r.eof = true
		}
		if len(ct) == 0 {
			return 0, io.EOF
		}
		pt, err := r.gcm.Open(nil, r.nonce, ct, nil)
		if err != nil {
			return 0, ErrRestoreDecryptFailed
		}
		r.data = pt
		r.eof = true
	}

	n := copy(p, r.data)
	r.data = r.data[n:]
	if len(r.data) == 0 && r.eof {
		return n, io.EOF
	}
	return n, nil
}
