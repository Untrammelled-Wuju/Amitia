package com.amitia.amitia_app.nativeprovider.root

data class RootCapabilityState(
    val supported: Boolean = false,
    val frameworkDetected: String? = null,
    val authorizationState: String = "unavailable",
    val suAvailable: Boolean = false,
    val userActionRequired: Boolean = false,
    val state: String = "host_unavailable",
    val reason: String = "android native host source not available",
)

data class RootExecuteRequest(
    val executable: String,
    val args: List<String> = emptyList(),
    val env: Map<String, String> = emptyMap(),
    val workDir: String? = null,
    val timeoutSeconds: Int = 30,
)

data class RootExecuteResult(
    val exitCode: Int = -1,
    val stdout: String = "",
    val stderr: String = "",
    val timedOut: Boolean = false,
)
