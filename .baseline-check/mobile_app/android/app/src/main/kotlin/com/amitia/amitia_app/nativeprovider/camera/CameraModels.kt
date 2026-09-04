package com.amitia.amitia_app.nativeprovider.camera

data class CameraCapabilityState(
    val supported: Boolean = false,
    val hasCamera: Boolean = false,
    val hasFrontCamera: Boolean = false,
    val hasBackCamera: Boolean = false,
    val camera2Level: String = "legacy",
    val state: String = "host_unavailable",
    val reason: String = "android native host source not available",
)

data class CameraInfo(
    val cameraId: String = "",
    val facing: String = "unknown",
    val supportedSizes: List<List<Int>> = emptyList(),
)

data class CameraCaptureResult(
    val captured: Boolean = false,
    val cameraId: String? = null,
    val width: Int = 0,
    val height: Int = 0,
    val mimeType: String = "image/jpeg",
    val filePath: String? = null,
)
