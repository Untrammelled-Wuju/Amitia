//go:build linux && !android

package archive

import (
	"compress/bzip2"
	"compress/gzip"
	"context"
	"io"
	"io/ioutil"
	"os"
)

func openDecompressor(ctx context.Context, file *os.File, format Format) (io.ReadCloser, error) {
	switch format {
	case FormatTARGZ, FormatGZIP:
		return gzip.NewReader(file)
	case FormatTARBZ2, FormatBZIP2:
		return ioutil.NopCloser(bzip2.NewReader(file)), nil
	case FormatTARXZ, FormatXZ:
		return openXZReader(file)
	case FormatTARZST, FormatZSTD:
		return openZstdReader(file)
	default:
		return nil, ErrFormatUnsupported(string(format))
	}
}

func openCompressor(ctx context.Context, file *os.File, format Format) (io.WriteCloser, func() error, error) {
	switch format {
	case FormatTARGZ, FormatGZIP:
		gzWriter, err := gzip.NewWriterLevel(file, gzip.DefaultCompression)
		if err != nil {
			return nil, nil, err
		}
		return gzWriter, func() error {
			if err := gzWriter.Close(); err != nil {
				return err
			}
			return file.Close()
		}, nil
	case FormatTARBZ2, FormatBZIP2:
		return nil, nil, ErrFormatUnsupported("bzip2 writing not supported in stdlib")
	case FormatTARXZ, FormatXZ:
		return nil, nil, ErrFormatUnsupported("xz writing not supported in stdlib")
	case FormatTARZST, FormatZSTD:
		return nil, nil, ErrFormatUnsupported("zstd writing not supported (no third-party library)")
	default:
		return nil, nil, ErrFormatUnsupported(string(format))
	}
}

func openXZReader(file *os.File) (io.ReadCloser, error) {
	return nil, ErrFormatUnsupported("xz decompression not supported in stdlib (needs third-party library)")
}

func openZstdReader(file *os.File) (io.ReadCloser, error) {
	return nil, ErrFormatUnsupported("zstd decompression not supported (needs third-party library)")
}

type wrappedReadCloser struct {
	io.Reader
	close func() error
}

func (w *wrappedReadCloser) Close() error {
	if w.close != nil {
		return w.close()
	}
	return nil
}
