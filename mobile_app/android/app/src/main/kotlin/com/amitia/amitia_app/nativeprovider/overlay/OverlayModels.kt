package com.amitia.amitia_app.nativeprovider.overlay

data class OverlayCapabilityState(
    val supported: Boolean = false,
    val permissionGranted: Boolean = false,
    val permissionRequired: Boolean = true,
    val canDrawOverlays: Boolean = false,
    val activeOverlays: Int = 0,
    val state: String = "host_unavailable",
    val reason: String = "android native host source not available",
)

data class OverlayCreateRequest(
    val overlayId: String = "",
    val overlayType: String = "frame",
    val width: Int = -1,
    val height: Int = -1,
    val x: Int = 0,
    val y: Int = 0,
    val focusable: Boolean = false,
    val clickThrough: Boolean = false,
    val transparent: Boolean = true,
)

data class OverlayInfo(
    val overlayId: String = "",
    val visible: Boolean = false,
    val width: Int = 0,
    val height: Int = 0,
    val x: Int = 0,
    val y: Int = 0,
    val generation: Long = 0L,
)

data class OverlayUpdateRequest(
    val overlayId: String = "",
    val payload: Map<String, Any?> = emptyMap(),
)
