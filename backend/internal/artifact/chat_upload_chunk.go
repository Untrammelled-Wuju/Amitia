package artifact

import (
	"context"
	"errors"
	"fmt"
	"io"
)

const ChatUploadChunkMaxBytes int64 = 512 << 10

type ChatUploadChunk struct {
	ArtifactID ID         `json:"artifactId"`
	Digest     BlobDigest `json:"digest"`
	Offset     int64      `json:"offset"`
	Size       int64      `json:"size"`
	Data       []byte     `json:"data"`
	EOF        bool       `json:"eof"`
}

func (s *Service) ReadChatUploadChunk(ctx context.Context, ownerSpaceID, ownerDeviceID, storageDeviceID string, id ID, offset, length int64) (chunk ChatUploadChunk, returnErr error) {
	if offset < 0 || length < 1 || length > ChatUploadChunkMaxBytes {
		return chunk, ErrInvalidReference("invalid chat upload range")
	}
	art, err := s.GetChatUploadReference(ctx, ownerSpaceID, ownerDeviceID, storageDeviceID, id)
	if err != nil {
		return chunk, err
	}
	if art.Source != SourceChatUpload || offset > art.SizeBytes {
		return chunk, ErrInvalidReference("chat upload range or source is invalid")
	}
	reader, info, err := s.OpenBlob(ctx, art.BlobDigest)
	if err != nil {
		return chunk, err
	}
	defer func() {
		if err := reader.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("artifact: close chat upload: %w", err))
		}
	}()
	if info.Digest != art.BlobDigest || info.SizeBytes != art.SizeBytes {
		return chunk, fmt.Errorf("artifact: stored chat upload metadata mismatch: expected digest %s size %d, got digest %s size %d", art.BlobDigest, art.SizeBytes, info.Digest, info.SizeBytes)
	}
	if _, err := reader.Seek(offset, io.SeekStart); err != nil {
		return chunk, fmt.Errorf("artifact: seek chat upload: %w", err)
	}
	remaining := art.SizeBytes - offset
	if length > remaining {
		length = remaining
	}
	data := make([]byte, int(length))
	if _, err := io.ReadFull(&contextReader{ctx: ctx, reader: reader}, data); err != nil {
		return chunk, fmt.Errorf("artifact: read chat upload range: %w", err)
	}
	return ChatUploadChunk{ArtifactID: id, Digest: art.BlobDigest, Offset: offset, Size: art.SizeBytes, Data: data, EOF: offset+length == art.SizeBytes}, nil
}
