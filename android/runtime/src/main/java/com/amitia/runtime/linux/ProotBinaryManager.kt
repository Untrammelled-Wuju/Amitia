package com.amitia.runtime.linux

import kotlinx.coroutines.flow.Flow
import java.io.File

interface ProotBinaryManager {

    fun isAvailable(): Boolean

    fun install(): Flow<ProotInstallProgress>

    fun version(): String?

    fun binaryPath(): File?

    fun verify(): Result<Unit>

    fun unavailableReason(): String?
}

data class ProotInstallProgress(
    val phase: ProotInstallPhase,
    val percent: Float,
    val message: String,
    val error: String? = null
)

enum class ProotInstallPhase {
    STARTED,
    COPYING,
    VERIFYING,
    COMPLETED,
    FAILED
}
