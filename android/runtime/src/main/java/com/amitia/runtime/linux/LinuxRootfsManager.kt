package com.amitia.runtime.linux

import kotlinx.coroutines.flow.Flow
import java.io.File

interface LinuxRootfsManager {

    fun rootfsDir(): File

    fun binDir(): File

    fun versionsDir(): File

    fun versionFile(): File

    suspend fun isInstalled(): Boolean

    suspend fun getCurrentVersion(): String?

    suspend fun getManifestVersion(): String?

    suspend fun install(): Flow<RootfsInstallProgress>

    suspend fun verify(): Result<RootfsIntegrity>

    suspend fun upgrade(): Flow<RootfsInstallProgress>

    suspend fun cleanup(requireConfirmation: Boolean): Result<Unit>

    suspend fun info(): RootfsInfo?

    data class RootfsInfo(
        val version: String,
        val manifestVersion: String,
        val installedAt: Long,
        val sizeBytes: Long,
        val fileCount: Int,
        val components: List<ComponentInfo>
    )

    data class ComponentInfo(
        val name: String,
        val file: String,
        val size: Long,
        val sha256: String,
        val target: String,
        val installed: Boolean
    )
}

data class RootfsInstallProgress(
    val phase: RootfsInstallPhase,
    val currentFile: String,
    val bytesCopied: Long,
    val totalBytes: Long,
    val percent: Float,
    val message: String,
    val error: String? = null
)

enum class RootfsInstallPhase {
    STARTED,
    COPYING,
    VERIFYING,
    FINALIZING,
    COMPLETED,
    FAILED
}
