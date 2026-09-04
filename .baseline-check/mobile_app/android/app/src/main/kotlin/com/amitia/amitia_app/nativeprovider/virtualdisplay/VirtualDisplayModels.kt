package com.amitia.amitia_app.nativeprovider.virtualdisplay

data class VirtualDisplayCapabilityState(
    val supported: Boolean = false,
    val state: String = "host_unavailable",
    val reason: String = "android native host source not available",
)

data class VirtualDisplayCreateRequest(
    val name: String = "amitia_virtual",
    val width: Int = 1080,
    val height: Int = 1920,
    val densityDpi: Int = 320,
)

data class VirtualDisplayInfo(
    val displayId: Int = -1,
    val name: String = "",
    val width: Int = 0,
    val height: Int = 0,
    val densityDpi: Int = 0,
    val rotation: Int = 0,
    val generation: Long = 0L,
    val active: Boolean = false,
)
