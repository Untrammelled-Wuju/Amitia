package media

type AuthorizationStatus string

const (
	AuthNotDetermined AuthorizationStatus = "not_determined"
	AuthDenied        AuthorizationStatus = "denied"
	AuthRestricted    AuthorizationStatus = "restricted"
	AuthLimited       AuthorizationStatus = "limited"
	AuthAuthorized    AuthorizationStatus = "authorized"
)

type MediaStatus struct {
	Supported            bool               `json:"supported"`
	PhotosPickerSupported bool              `json:"photosPickerSupported"`
	PhotoLibraryReadWrite AuthorizationStatus `json:"photoLibraryReadWrite"`
	PhotoLibraryAddOnly  AuthorizationStatus `json:"photoLibraryAddOnly"`
	CameraAuthorization  AuthorizationStatus `json:"cameraAuthorization"`
	MicrophoneAuthorization AuthorizationStatus `json:"microphoneAuthorization"`
	CameraAvailable      bool               `json:"cameraAvailable"`
	MicrophoneAvailable  bool               `json:"microphoneAvailable"`
	CanCapturePhoto      bool               `json:"canCapturePhoto"`
	CanRecordVideo       bool               `json:"canRecordVideo"`
	CanRecordAudio       bool               `json:"canRecordAudio"`
	Generation           uint64             `json:"generation"`
}

type MediaPickerRequest struct {
	Kinds            []string `json:"kinds"`
	SelectionLimit   int      `json:"selectionLimit"`
	Ordered          bool     `json:"ordered"`
	PreferredEncoding string  `json:"preferredEncoding,omitempty"`
	MaxTotalBytes    int64    `json:"maxTotalBytes"`
}

type MediaResource struct {
	ResourceURI  string `json:"resourceUri"`
	MediaType    string `json:"mediaType"`
	MIMEType     string `json:"mimeType"`
	Filename     string `json:"filename,omitempty"`
	SizeBytes    int64  `json:"sizeBytes"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
	DurationMs   int64  `json:"durationMs,omitempty"`
	ContentHash  string `json:"contentHash,omitempty"`
	Source       string `json:"source"`
}

type MediaPickerResult struct {
	Items      []MediaResource `json:"items"`
	Cancelled  bool            `json:"cancelled"`
	TotalBytes int64           `json:"totalBytes"`
}

type PhotoListRequest struct {
	MediaType    string  `json:"mediaType,omitempty"`
	CreatedAfter *string `json:"createdAfter,omitempty"`
	CreatedBefore *string `json:"createdBefore,omitempty"`
	Favorite     *bool   `json:"favorite,omitempty"`
	Limit        int     `json:"limit"`
	Cursor       string  `json:"cursor,omitempty"`
	Sort         string  `json:"sort,omitempty"`
}

type PhotoAssetInfo struct {
	AssetRef      string   `json:"assetRef"`
	MediaType     string   `json:"mediaType"`
	MediaSubtypes []string `json:"mediaSubtypes,omitempty"`
	Width         int      `json:"width,omitempty"`
	Height        int      `json:"height,omitempty"`
	DurationMs    int64    `json:"durationMs,omitempty"`
	CreatedAt     string   `json:"createdAt,omitempty"`
	ModifiedAt    string   `json:"modifiedAt,omitempty"`
	Favorite      bool     `json:"favorite"`
	Hidden        bool     `json:"hidden"`
	SourceType    []string `json:"sourceType,omitempty"`
	Generation    uint64   `json:"generation"`
}

type PhotoAssetListResult struct {
	Assets     []PhotoAssetInfo `json:"assets"`
	NextCursor string           `json:"nextCursor"`
	HasMore    bool             `json:"hasMore"`
	Generation uint64           `json:"generation"`
}

type PhotoExportRequest struct {
	AssetRef       string `json:"assetRef"`
	Representation string `json:"representation"`
	NetworkAccess  bool   `json:"networkAccess"`
	MaxBytes       int64  `json:"maxBytes"`
}

type PhotoSaveRequest struct {
	ResourceURI      string `json:"resourceUri"`
	MediaType        string `json:"mediaType"`
	AlbumRef         string `json:"albumRef,omitempty"`
	PreserveMetadata bool   `json:"preserveMetadata"`
}

type PhotoSaveResult struct {
	Saved           bool   `json:"saved"`
	AssetRef        string `json:"assetRef,omitempty"`
	ChangeCount     int    `json:"changeCount"`
	PhotosSaveSucceeded bool `json:"photosSaveSucceeded"`
	Generation      uint64 `json:"generation"`
}

type PhotoDeleteRequest struct {
	AssetRefs []string `json:"assetRefs"`
}

type PhotoDeleteResult struct {
	Deleted       bool     `json:"deleted"`
	DeletedRefs   []string `json:"deletedRefs"`
	FailedRefs    []string `json:"failedRefs"`
	ChangeCount   int      `json:"changeCount"`
}

type CameraDevice struct {
	DeviceRef      string  `json:"deviceRef"`
	Position       string  `json:"position"`
	DeviceType     string  `json:"deviceType"`
	SupportsPhoto  bool    `json:"supportsPhoto"`
	SupportsVideo  bool    `json:"supportsVideo"`
	MinZoom        float64 `json:"minZoom,omitempty"`
	MaxZoom        float64 `json:"maxZoom,omitempty"`
	HasFlash       bool    `json:"hasFlash"`
	HasTorch       bool    `json:"hasTorch"`
}

type CameraCaptureRequest struct {
	DeviceRef        string `json:"deviceRef,omitempty"`
	Quality          string `json:"quality,omitempty"`
	Flash            string `json:"flash,omitempty"`
	Format           string `json:"format,omitempty"`
	MirrorFrontCamera bool  `json:"mirrorFrontCamera,omitempty"`
	SaveToPhotos     bool   `json:"saveToPhotos,omitempty"`
}

type CameraCaptureResult struct {
	ResourceURI       string `json:"resourceUri"`
	MediaType         string `json:"mediaType"`
	MIMEType          string `json:"mimeType"`
	SizeBytes         int64  `json:"sizeBytes"`
	Width             int    `json:"width,omitempty"`
	Height            int    `json:"height,omitempty"`
	ContentHash       string `json:"contentHash,omitempty"`
	FlashUsed         string `json:"flashUsed,omitempty"`
	PhotosSaveSucceeded bool  `json:"photosSaveSucceeded"`
}

type VideoRecordRequest struct {
	DeviceRef    string `json:"deviceRef,omitempty"`
	IncludeAudio bool   `json:"includeAudio"`
	MaxDurationMs int64 `json:"maxDurationMs"`
	Quality      string `json:"quality,omitempty"`
	Torch        string `json:"torch,omitempty"`
	SaveToPhotos bool   `json:"saveToPhotos,omitempty"`
}

type VideoRecordResult struct {
	ResourceURI       string `json:"resourceUri"`
	MediaType         string `json:"mediaType"`
	MIMEType          string `json:"mimeType"`
	SizeBytes         int64  `json:"sizeBytes"`
	DurationMs        int64  `json:"durationMs"`
	Width             int    `json:"width,omitempty"`
	Height            int    `json:"height,omitempty"`
	AudioIncluded     bool   `json:"audioIncluded"`
	Interrupted       bool   `json:"interrupted"`
	PhotosSaveSucceeded bool  `json:"photosSaveSucceeded"`
}

type AudioRecordRequest struct {
	Format       string `json:"format,omitempty"`
	SampleRate   int    `json:"sampleRate,omitempty"`
	Channels     int    `json:"channels,omitempty"`
	Quality      string `json:"quality,omitempty"`
	MaxDurationMs int64 `json:"maxDurationMs"`
}

type AudioRecordResult struct {
	ResourceURI   string `json:"resourceUri"`
	MediaType     string `json:"mediaType"`
	MIMEType      string `json:"mimeType"`
	SizeBytes     int64  `json:"sizeBytes"`
	DurationMs    int64  `json:"durationMs"`
	SampleRate    int    `json:"sampleRate"`
	Channels      int    `json:"channels"`
	Interrupted   bool   `json:"interrupted"`
}
