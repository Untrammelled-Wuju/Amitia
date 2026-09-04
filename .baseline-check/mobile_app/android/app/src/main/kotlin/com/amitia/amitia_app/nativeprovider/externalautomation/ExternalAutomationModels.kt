package com.amitia.amitia_app.nativeprovider.externalautomation

data class ExternalAutomationCapabilityState(
    val supported: Boolean = false,
    val canLaunchApps: Boolean = false,
    val canLaunchUris: Boolean = false,
    val canOpenSettings: Boolean = false,
    val state: String = "host_unavailable",
    val reason: String = "android native host source not available",
)

data class ResolveAppRequest(
    val query: String = "",
    val byPackage: Boolean = true,
    val byLabel: Boolean = true,
)

data class ResolvedApp(
    val packageName: String = "",
    val label: String = "",
    val mainActivity: String? = null,
    val installed: Boolean = true,
)

data class OpenAppRequest(
    val packageName: String = "",
    val activityName: String? = null,
    val bringToFront: Boolean = true,
)

data class ForegroundState(
    val packageName: String = "",
    val activityName: String? = null,
    val isForeground: Boolean = false,
)
