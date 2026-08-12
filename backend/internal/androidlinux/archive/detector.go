//go:build linux && !android

package archive

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Detector struct {
	fileOpener func(path string) (*os.File, error)
}

func NewDetector() *Detector {
	return &Detector{
		fileOpener: func(path string) (*os.File, error) {
			return os.Open(path)
		},
	}
}

func (d *Detector) SetFileOpener(opener func(path string) (*os.File, error)) {
	d.fileOpener = opener
}

func (d *Detector) Detect(path string) (DetectResult, error) {
	result := DetectResult{Path: path}

	info, err := os.Stat(path)
	if err != nil {
		return result, ErrReadFailed(err.Error())
	}
	result.SizeBytes = info.Size()

	if info.IsDir() {
		return result, ErrNotArchive(path)
	}

	file, err := d.fileOpener(path)
	if err != nil {
		return result, ErrReadFailed(err.Error())
	}
	defer file.Close()

	magic := make([]byte, 512)
	n, err := io.ReadFull(file, magic)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return result, ErrReadFailed(err.Error())
	}
	magic = magic[:n]

	format := detectByMagic(magic)
	if format == "" {
		format = detectByExtension(path)
	}

	if format == "" {
		return result, nil
	}

	result.Format = format
	result.Compressed = isCompressedFormat(format)
	result.Archive = isArchiveFormat(format)

	if format == FormatZIP {
		result.Encrypted = detectEncrypted(magic)
	}

	result.MIMEType = formatToMIME(format)
	return result, nil
}

func detectByMagic(magic []byte) Format {
	if len(magic) < 4 {
		return ""
	}

	if bytes.HasPrefix(magic, []byte("PK\x03\x04")) || bytes.HasPrefix(magic, []byte("PK\x05\x06")) || bytes.HasPrefix(magic, []byte("PK\x07\x08")) {
		return FormatZIP
	}
	if bytes.HasPrefix(magic, []byte{0x1f, 0x8b, 0x08}) {
		return FormatTARGZ
	}
	if bytes.HasPrefix(magic, []byte("BZ")) && len(magic) > 3 && magic[2] == 'h' {
		return FormatTARBZ2
	}
	if bytes.HasPrefix(magic, []byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00}) {
		return FormatTARXZ
	}
	if len(magic) >= 4 && magic[0] == 0x28 && magic[1] == 0xb5 && magic[2] == 0x2f && magic[3] == 0xfd {
		return FormatTARZST
	}
	if isTarHeader(magic) {
		return FormatTAR
	}

	return ""
}

func isTarHeader(b []byte) bool {
	if len(b) < 512 {
		return false
	}
	ustar := b[257:263]
	if !bytes.Equal(ustar, []byte("ustar\x00")) && !bytes.Equal(ustar, []byte("ustar ")) {
		return false
	}
	chksum := b[148:156]
	hasNonSpace := false
	for _, c := range chksum {
		if c != ' ' && c != 0 {
			hasNonSpace = true
			break
		}
	}
	return hasNonSpace
}

func detectByExtension(path string) Format {
	base := strings.ToLower(path)

	switch {
	case strings.HasSuffix(base, ".tar.gz"), strings.HasSuffix(base, ".tgz"):
		return FormatTARGZ
	case strings.HasSuffix(base, ".tar.bz2"), strings.HasSuffix(base, ".tbz2"):
		return FormatTARBZ2
	case strings.HasSuffix(base, ".tar.xz"), strings.HasSuffix(base, ".txz"):
		return FormatTARXZ
	case strings.HasSuffix(base, ".tar.zst"), strings.HasSuffix(base, ".tzst"):
		return FormatTARZST
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".zip":
		return FormatZIP
	case ".tar":
		return FormatTAR
	case ".gz":
		return FormatGZIP
	case ".bz2":
		return FormatBZIP2
	case ".xz":
		return FormatXZ
	case ".zst":
		return FormatZSTD
	}

	return ""
}

func isCompressedFormat(f Format) bool {
	switch f {
	case FormatGZIP, FormatBZIP2, FormatXZ, FormatZSTD, FormatTARGZ, FormatTARBZ2, FormatTARXZ, FormatTARZST:
		return true
	}
	return false
}

func isArchiveFormat(f Format) bool {
	switch f {
	case FormatZIP, FormatTAR, FormatTARGZ, FormatTARBZ2, FormatTARXZ, FormatTARZST:
		return true
	}
	return false
}

func detectEncrypted(magic []byte) bool {
	return len(magic) > 6 && magic[6]&0x01 != 0
}

func formatToMIME(format Format) string {
	switch format {
	case FormatZIP:
		return "application/zip"
	case FormatTAR:
		return "application/x-tar"
	case FormatTARGZ, FormatGZIP:
		return "application/gzip"
	case FormatTARBZ2, FormatBZIP2:
		return "application/x-bzip2"
	case FormatTARXZ, FormatXZ:
		return "application/x-xz"
	case FormatTARZST, FormatZSTD:
		return "application/zstd"
	}
	return "application/octet-stream"
}
