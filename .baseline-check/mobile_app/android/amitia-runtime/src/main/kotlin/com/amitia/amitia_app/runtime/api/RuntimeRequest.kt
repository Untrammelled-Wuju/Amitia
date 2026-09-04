package com.amitia.amitia_app.runtime.api

data class RuntimeInstallRequest(
    val packageUri: String,
    val expectedVersion: String?,
    val allowRepairExisting: Boolean
) {
    init {
        require(packageUri.trim().isNotEmpty()) { "packageUri must not be blank" }
    }
}

data class RuntimeVerifyRequest(
    val deep: Boolean
)

enum class RuntimeStartReason {
    APP_LAUNCH,
    USER_REQUEST,
    RECOVERY,
    BACKGROUND_TASK
}

data class RuntimeStartRequest(
    val reason: RuntimeStartReason,
    val profile: String = "local"
)

enum class RuntimeStopReason {
    USER_REQUEST,
    APP_SHUTDOWN,
    RESTART,
    RECOVERY,
    INSTALL_REPLACEMENT
}

data class RuntimeStopRequest(
    val reason: RuntimeStopReason,
    val force: Boolean
)

data class RuntimeRepairRequest(
    val packageUri: String?,
    val preserveUserData: Boolean = true
)
