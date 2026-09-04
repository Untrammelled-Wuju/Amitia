package com.amitia.amitia_app.nativeprovider.shizuku

data class ShizukuCommandResult(
    val exitCode: Int = -1,
    val exitCodeAvailable: Boolean = false,
    val stdout: String = "",
    val stderr: String = "",
    val durationMs: Long = 0,
    val timedOut: Boolean = false,
)

data class ShizukuCommandRequest(
    val executable: String,
    val args: List<String> = emptyList(),
    val stdin: String? = null,
    val timeoutMs: Long = 30000,
    val maxOutputBytes: Long = 1048576,
)
