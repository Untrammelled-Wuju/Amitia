package com.amitia.amitia_app.nativeprovider.shizuku

data class ShizukuCapabilityState(
    val supported: Boolean = false,
    val installed: Boolean = false,
    val binderAvailable: Boolean = false,
    val permissionState: String = "unavailable",
    val state: String = "unavailable",
    val reason: String = "shizuku not available",
)
