package management

type PackageInstallRequest struct {
	ArchivePath string `json:"archivePath" binding:"required"`
}

type PackageUpdateBody struct {
	ArchivePath string `json:"archivePath"`
}

type PackageUpdateRequest struct {
	ExtensionID string `json:"-"`
	ArchivePath string `json:"archivePath"`
}

type PackageEnableRequest struct {
	ExtensionID string `json:"extensionId" binding:"required"`
}

type PackageDisableRequest struct {
	ExtensionID string `json:"extensionId" binding:"required"`
}

type PackageUninstallRequest struct {
	ExtensionID string `json:"extensionId" binding:"required"`
}

type RuntimeStartRequest struct {
	RuntimeID string `json:"runtimeId" binding:"required"`
}

type RuntimeStopRequest struct {
	RuntimeID string `json:"runtimeId" binding:"required"`
}

type RuntimeRestartRequest struct {
	RuntimeID string `json:"runtimeId" binding:"required"`
}

type PackageMutationResult struct {
	ExtensionID    string `json:"extensionId"`
	Operation      string `json:"operation"`
	State          string `json:"state"`
	CurrentVersion string `json:"currentVersion,omitempty"`
	Warnings       string `json:"warnings,omitempty"`
}
