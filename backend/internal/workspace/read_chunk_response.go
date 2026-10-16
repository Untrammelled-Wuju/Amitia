package workspace

import "encoding/base64"

type ReadChunkResponse struct {
	Resource      string `json:"resource"`
	IsText        bool   `json:"isText"`
	ContentBase64 string `json:"contentBase64"`
	BytesRead     int64  `json:"bytesRead"`
	NextOffset    int64  `json:"nextOffset"`
	HasMore       bool   `json:"hasMore"`
}

func buildReadChunkResponse(result ReadResult, offset, requestedBytes int64) ReadChunkResponse {
	length := int64(len(result.Content))
	return ReadChunkResponse{
		Resource:      result.Resource,
		IsText:        result.IsText,
		ContentBase64: base64.StdEncoding.EncodeToString(result.Content),
		BytesRead:     length,
		NextOffset:    offset + length,
		HasMore:       length > 0 && length >= requestedBytes,
	}
}
