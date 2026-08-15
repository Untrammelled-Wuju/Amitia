package artifact

import "fmt"

type ArtifactError struct {
	Code string
	Msg  string
}

func (e *ArtifactError) Error() string {
	return fmt.Sprintf("artifact.%s: %s", e.Code, e.Msg)
}

func ErrInvalidUpload(msg string) *ArtifactError {
	return &ArtifactError{Code: "invalid_upload", Msg: msg}
}

func ErrTooLarge(max int64) *ArtifactError {
	return &ArtifactError{Code: "too_large", Msg: fmt.Sprintf("exceeds limit %d", max)}
}

func ErrUnsupportedMIME(mime string) *ArtifactError {
	return &ArtifactError{Code: "unsupported_mime", Msg: mime}
}

func ErrNotFound(id ID) *ArtifactError {
	return &ArtifactError{Code: "not_found", Msg: string(id)}
}

func ErrNotOwned(id ID) *ArtifactError {
	return &ArtifactError{Code: "not_owned", Msg: string(id)}
}

func ErrDeleted(id ID) *ArtifactError {
	return &ArtifactError{Code: "deleted", Msg: string(id)}
}

func ErrInUse(id ID) *ArtifactError {
	return &ArtifactError{Code: "in_use", Msg: string(id)}
}

func ErrBlobMissing(digest BlobDigest) *ArtifactError {
	return &ArtifactError{Code: "blob_missing", Msg: string(digest)}
}

func ErrBlobWriteFailed(err error) *ArtifactError {
	return &ArtifactError{Code: "blob_write_failed", Msg: err.Error()}
}

func ErrMetadataWriteFailed(err error) *ArtifactError {
	return &ArtifactError{Code: "metadata_write_failed", Msg: err.Error()}
}

func ErrInvalidReference(msg string) *ArtifactError {
	return &ArtifactError{Code: "invalid_reference", Msg: msg}
}
