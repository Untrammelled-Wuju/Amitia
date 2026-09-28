package com.amitia.amitia_app.nativeprovider.shizuku

data class ShizukuCapabilityState(
    val supported: Boolean = true,
    val installed: Boolean = false,
    val managerInstalled: Boolean = false,
    val binderAvailable: Boolean = false,
    val permissionState: String = "unavailable",
    val state: String = "unavailable",
    val reason: String = "shizuku not available",
    val enabled: Boolean = false,
    val provider: String = "none",
    val version: Int = -1,
    val uid: Int = -1,
    val canRequestPermission: Boolean = false,
)
