package background

type WorkspaceKind string

const (
	WorkspaceKindLocal    WorkspaceKind = "local"
	WorkspaceKindSAF      WorkspaceKind = "saf"
	WorkspaceKindIOSDocument WorkspaceKind = "ios_document"
)

type FileProviderHint string

const (
	FileProviderICloud      FileProviderHint = "icloud"
	FileProviderOnMyIPhone  FileProviderHint = "on_my_iphone"
	FileProviderDropbox     FileProviderHint = "dropbox"
	FileProviderOneDrive    FileProviderHint = "onedrive"
	FileProviderUnknown     FileProviderHint = "unknown"
)

type FileMountStatus string

const (
	FileMountStatusAvailable    FileMountStatus = "available"
	FileMountStatusUnavailable  FileMountStatus = "unavailable"
	FileMountStatusMissing      FileMountStatus = "missing"
	FileMountStatusPermissionRevoked FileMountStatus = "permission_revoked"
	FileMountStatusUnsupported  FileMountStatus = "unsupported"
)

type IOSFileImportResult struct {
	ResourceURI string `json:"resourceUri"`
	DisplayName string `json:"displayName"`
	MIMEType    string `json:"mimeType,omitempty"`
	SizeBytes   int64  `json:"sizeBytes"`
	SourceKind  string `json:"sourceKind"`
}

type IOSDirectoryMountResult struct {
	WorkspaceID string `json:"workspaceId"`
	RootURI     string `json:"rootUri"`
	DisplayName string `json:"displayName"`
	ReadOnly    bool   `json:"readOnly"`
	Status      string `json:"status"`
}

type IOSFileAccessRequest struct {
	MountID    string `json:"mountId"`
	RelativePath string `json:"relativePath"`
}

type IOSFileReadRequest struct {
	MountID      string `json:"mountId"`
	RelativePath string `json:"relativePath"`
	Offset       int64  `json:"offset,omitempty"`
	Length       int64  `json:"length,omitempty"`
}

type IOSFileWriteRequest struct {
	MountID      string `json:"mountId"`
	RelativePath string `json:"relativePath"`
	Content      []byte `json:"content"`
	Atomic       bool   `json:"atomic"`
}

type IOSFileMkdirRequest struct {
	MountID      string `json:"mountId"`
	RelativePath string `json:"relativePath"`
}

type IOSFileRenameRequest struct {
	MountID      string `json:"mountId"`
	RelativePath string `json:"relativePath"`
	NewName      string `json:"newName"`
}

type IOSFileMoveRequest struct {
	MountID          string `json:"mountId"`
	RelativePath     string `json:"relativePath"`
	NewRelativePath  string `json:"newRelativePath"`
}

type IOSFileCopyRequest struct {
	MountID          string `json:"mountId"`
	RelativePath     string `json:"relativePath"`
	NewRelativePath  string `json:"newRelativePath"`
}

type IOSFileDeleteRequest struct {
	MountID      string `json:"mountId"`
	RelativePath string `json:"relativePath"`
}

type IOSFileStatResult struct {
	Name         string `json:"name"`
	SizeBytes    int64  `json:"sizeBytes"`
	IsDirectory  bool   `json:"isDirectory"`
	IsUbiquitous bool   `json:"isUbiquitous"`
	IsMaterialized bool `json:"isMaterialized"`
	ModifiedAt   string `json:"modifiedAt,omitempty"`
	DisplayName  string `json:"displayName,omitempty"`
	MIMEType     string `json:"mimeType,omitempty"`
}

type IOSFileListResult struct {
	Entries []IOSFileStatResult `json:"entries"`
	Count   int                 `json:"count"`
}

type IOSFileReadResult struct {
	Content  []byte `json:"content"`
	Size     int64  `json:"size"`
	Finished bool   `json:"finished"`
}

type IOSFileExportRequest struct {
	MountID      string `json:"mountId"`
	RelativePath string `json:"relativePath"`
	ResourceURI  string `json:"resourceUri"`
}

type IOSFileMountResult struct {
	WorkspaceID    string           `json:"workspaceId"`
	RootURI        string           `json:"rootUri"`
	DisplayName    string           `json:"displayName"`
	ReadOnly       bool             `json:"readOnly"`
	Status         FileMountStatus  `json:"status"`
	ProviderHint   FileProviderHint `json:"providerHint,omitempty"`
}

type IOSFileCapabilities struct {
	AtomicWrite      bool `json:"atomicWrite"`
	Rename           bool `json:"rename"`
	Move             bool `json:"move"`
	Copy             bool `json:"copy"`
	Delete           bool `json:"delete"`
	CoordinatedAccess bool `json:"coordinatedAccess"`
}

type IOSFileMountReauthorizeRequest struct {
	MountID string `json:"mountId"`
}

type IOSFileGetCapabilitiesRequest struct {
	MountID string `json:"mountId"`
}
