package media

const (
	OperationStatus           = "media.status"
	OperationPhotosPick       = "media.photos.pick"
	OperationPhotosStatus     = "media.photos.status"
	OperationPhotosList       = "media.photos.list"
	OperationPhotosGet        = "media.photos.get"
	OperationPhotosExport     = "media.photos.export"
	OperationPhotosSave       = "media.photos.save"
	OperationPhotosDelete     = "media.photos.delete"
	OperationPhotosManageLimited = "media.photos.manage_limited_access"
	OperationCameraStatus     = "media.camera.status"
	OperationCameraDevices    = "media.camera.devices"
	OperationCameraCapturePhoto = "media.camera.capture_photo"
	OperationCameraRecordVideo = "media.camera.record_video"
	OperationAudioStatus      = "media.audio.status"
	OperationAudioRecord      = "media.audio.record"
	OperationStagingImport    = "media.staging.import"
)

func Operations() []string {
	return []string{
		OperationStatus,
		OperationPhotosPick,
		OperationPhotosStatus,
		OperationPhotosList,
		OperationPhotosGet,
		OperationPhotosExport,
		OperationPhotosSave,
		OperationPhotosDelete,
		OperationPhotosManageLimited,
		OperationCameraStatus,
		OperationCameraDevices,
		OperationCameraCapturePhoto,
		OperationCameraRecordVideo,
		OperationAudioStatus,
		OperationAudioRecord,
		OperationStagingImport,
	}
}
