package com.amitia.amitia_app.runtime.install.internal

import com.amitia.amitia_app.runtime.install.ActiveProgramRoot
import com.amitia.amitia_app.runtime.install.ActiveProgramRootResult
import com.amitia.amitia_app.runtime.install.ActiveRuntimeManager
import com.amitia.amitia_app.runtime.install.ActiveRuntimeResult
import com.amitia.amitia_app.runtime.install.RuntimeHostLayout
import com.amitia.amitia_app.runtime.install.RuntimeInstallErrorCode
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestStore
import java.io.File

internal class DefaultActiveRuntimeManager(
    private val layout: RuntimeHostLayout,
    private val manifestStore: RuntimeManifestStore? = null,
) : ActiveRuntimeManager {

    override fun current(): ActiveRuntimeResult {
        val markerFile = layout.activeRuntimeFile
        if (!markerFile.exists()) {
            return ActiveRuntimeResult.NoActiveRuntime
        }

        val content = try {
            markerFile.readText(Charsets.UTF_8)
        } catch (e: Exception) {
            return ActiveRuntimeResult.Failure(
                code = RuntimeInstallErrorCode.ACTIVE_RUNTIME_METADATA_INVALID,
                message = "failed to read active runtime marker: ${e.message}"
            )
        }

        val version = try {
            parseRuntimeVersion(content)
        } catch (e: Exception) {
            return ActiveRuntimeResult.Failure(
                code = RuntimeInstallErrorCode.ACTIVE_RUNTIME_METADATA_INVALID,
                message = "invalid active runtime marker: ${e.message}"
            )
        }

        val versionDir = layout.runtimeVersionRoot(version)
        if (!versionDir.exists() || !versionDir.isDirectory) {
            return ActiveRuntimeResult.Failure(
                code = RuntimeInstallErrorCode.ACTIVE_RUNTIME_MISSING,
                message = "active runtime version directory missing: $version"
            )
        }

        return ActiveRuntimeResult.Active(
            com.amitia.amitia_app.runtime.install.ActiveRuntimeInfo(
                version = version,
                activatedAtEpochMillis = 0L,
            )
        )
    }

    override fun activate(version: String): ActiveRuntimeResult {
        val validationError = validateVersionForActivation(version)
        if (validationError != null) {
            return validationError
        }

        val versionDir = layout.runtimeVersionRoot(version)
        if (!versionDir.exists() || !versionDir.isDirectory) {
            return ActiveRuntimeResult.Failure(
                code = RuntimeInstallErrorCode.ACTIVE_RUNTIME_MISSING,
                message = "cannot activate missing version: $version"
            )
        }

        if (manifestStore != null) {
            val manifestError = verifyManifestForActivation(version, versionDir)
            if (manifestError != null) {
                return manifestError
            }
        }

        val markerContent = buildMarkerContent(version)
        val markerFile = layout.activeRuntimeFile

        return try {
            markerFile.parentFile?.mkdirs()
            val tempFile = File("${markerFile.absolutePath}.tmp")
            tempFile.writeText(markerContent, Charsets.UTF_8)
            if (!tempFile.renameTo(markerFile)) {
                markerFile.delete()
                if (!tempFile.renameTo(markerFile)) {
                    tempFile.delete()
                    return ActiveRuntimeResult.Failure(
                        code = RuntimeInstallErrorCode.ACTIVE_RUNTIME_WRITE_FAILED,
                        message = "failed to atomically write active runtime marker"
                    )
                }
            }

            ActiveRuntimeResult.Active(
                com.amitia.amitia_app.runtime.install.ActiveRuntimeInfo(
                    version = version,
                    activatedAtEpochMillis = 0L,
                )
            )
        } catch (e: Exception) {
            ActiveRuntimeResult.Failure(
                code = RuntimeInstallErrorCode.ACTIVE_RUNTIME_WRITE_FAILED,
                message = "failed to write active runtime marker: ${e.message}"
            )
        }
    }

    private fun verifyManifestForActivation(
        version: String,
        versionDir: File,
    ): ActiveRuntimeResult.Failure? {
        val result = manifestStore!!.read()
        when (result) {
            is com.amitia.amitia_app.runtime.manifest.RuntimeManifestResult.Success -> {
                if (result.manifest.runtimeVersion != version) {
                    return ActiveRuntimeResult.Failure(
                        code = RuntimeInstallErrorCode.ACTIVE_RUNTIME_NOT_INSTALLED,
                        message = "manifest version ${result.manifest.runtimeVersion} does not match active version $version"
                    )
                }
            }
            is com.amitia.amitia_app.runtime.manifest.RuntimeManifestResult.Failure -> {
                return ActiveRuntimeResult.Failure(
                    code = RuntimeInstallErrorCode.ACTIVE_RUNTIME_NOT_INSTALLED,
                    message = "no valid installed manifest found: ${result.error.message}"
                )
            }
        }

        val backendFile = File(versionDir, "backend/amitia-server")
        if (!backendFile.exists()) {
            return ActiveRuntimeResult.Failure(
                code = RuntimeInstallErrorCode.ACTIVE_RUNTIME_MISSING,
                message = "backend entry missing: backend/amitia-server"
            )
        }
        val nodeFile = File(versionDir, "node/bin/node")
        if (!nodeFile.exists()) {
            return ActiveRuntimeResult.Failure(
                code = RuntimeInstallErrorCode.ACTIVE_RUNTIME_MISSING,
                message = "node entry missing: node/bin/node"
            )
        }
        val qdrantFile = File(versionDir, "qdrant/bin/qdrant")
        if (!qdrantFile.exists()) {
            return ActiveRuntimeResult.Failure(
                code = RuntimeInstallErrorCode.ACTIVE_RUNTIME_MISSING,
                message = "qdrant entry missing: qdrant/bin/qdrant"
            )
        }

        return null
    }

    override fun resolveActiveProgramRoot(): ActiveProgramRootResult {
        val markerFile = layout.activeRuntimeFile
        if (!markerFile.exists()) {
            return ActiveProgramRootResult.NoActiveRuntime
        }

        val content = try {
            markerFile.readText(Charsets.UTF_8)
        } catch (e: Exception) {
            return ActiveProgramRootResult.Failure(
                code = RuntimeInstallErrorCode.ACTIVE_RUNTIME_METADATA_INVALID,
                message = "failed to read active runtime marker: ${e.message}"
            )
        }

        val version = try {
            parseRuntimeVersion(content)
        } catch (e: Exception) {
            return ActiveProgramRootResult.Failure(
                code = RuntimeInstallErrorCode.ACTIVE_RUNTIME_METADATA_INVALID,
                message = "invalid active runtime marker: ${e.message}"
            )
        }

        val versionDir = layout.runtimeVersionRoot(version)
        if (!versionDir.exists() || !versionDir.isDirectory) {
            return ActiveProgramRootResult.Failure(
                code = RuntimeInstallErrorCode.ACTIVE_RUNTIME_MISSING,
                message = "active runtime version directory missing: $version"
            )
        }

        if (!versionDir.absolutePath.startsWith(layout.versionsRoot.absolutePath + "/")) {
            return ActiveProgramRootResult.Failure(
                code = RuntimeInstallErrorCode.ACTIVE_RUNTIME_MISSING,
                message = "active program root must be within versionsRoot"
            )
        }

        if (versionDir.absolutePath == layout.versionsRoot.absolutePath) {
            return ActiveProgramRootResult.Failure(
                code = RuntimeInstallErrorCode.ACTIVE_RUNTIME_MISSING,
                message = "active program root must not be versionsRoot directly"
            )
        }

        val manifestIdentity = try {
            val canon = versionDir.canonicalFile
            val versionsCanon = layout.versionsRoot.canonicalFile
            if (!canon.absolutePath.startsWith(versionsCanon.absolutePath + "/")) {
                return ActiveProgramRootResult.Failure(
                    code = RuntimeInstallErrorCode.ACTIVE_RUNTIME_MISSING,
                    message = "active program root symlink escape detected"
                )
            }
            canon.absolutePath
        } catch (e: Exception) {
            return ActiveProgramRootResult.Failure(
                code = RuntimeInstallErrorCode.ACTIVE_RUNTIME_MISSING,
                message = "failed to resolve canonical path: ${e.message}"
            )
        }

        if (manifestStore != null) {
            val storeResult = manifestStore.read()
            when (storeResult) {
                is com.amitia.amitia_app.runtime.manifest.RuntimeManifestResult.Success -> {
                    if (storeResult.manifest.runtimeVersion != version) {
                        return ActiveProgramRootResult.Failure(
                            code = RuntimeInstallErrorCode.ACTIVE_RUNTIME_NOT_INSTALLED,
                            message = "manifest version ${storeResult.manifest.runtimeVersion} does not match active version $version"
                        )
                    }
                }
                is com.amitia.amitia_app.runtime.manifest.RuntimeManifestResult.Failure -> {
                    return ActiveProgramRootResult.Failure(
                        code = RuntimeInstallErrorCode.ACTIVE_RUNTIME_NOT_INSTALLED,
                        message = "no valid installed manifest: ${storeResult.error.message}"
                    )
                }
            }
        }

        return ActiveProgramRootResult.Ready(
            ActiveProgramRoot(
                runtimeVersion = version,
                hostDirectory = versionDir,
                manifestIdentity = manifestIdentity,
            )
        )
    }

    private fun parseRuntimeVersion(content: String): String {
        val trimmed = content.trim()
        if (trimmed.isEmpty()) {
            throw IllegalArgumentException("empty marker content")
        }

        val schemaVersion = extractJsonInt(content, "schemaVersion")
        if (schemaVersion != null && schemaVersion != MARKER_SCHEMA_VERSION) {
            throw IllegalArgumentException("unsupported schema version: $schemaVersion")
        }

        val version = extractJsonString(content, "runtimeVersion")
        if (version.isNullOrBlank()) {
            throw IllegalArgumentException("missing runtimeVersion")
        }

        return version
    }

    private fun extractJsonString(json: String, key: String): String? {
        val pattern = Regex("\"$key\"\\s*:\\s*\"([^\"]+)\"")
        return pattern.find(json)?.groupValues?.get(1)
    }

    private fun extractJsonInt(json: String, key: String): Int? {
        val pattern = Regex("\"$key\"\\s*:\\s*(\\d+)")
        return pattern.find(json)?.groupValues?.get(1)?.toIntOrNull()
    }

    private fun buildMarkerContent(version: String): String {
        return buildString {
            appendLine("{")
            appendLine("  \"schemaVersion\": $MARKER_SCHEMA_VERSION,")
            appendLine("  \"runtimeVersion\": \"$version\"")
            append("}")
        }
    }

    private fun validateVersionForActivation(version: String): ActiveRuntimeResult.Failure? {
        if (version.isBlank()) {
            return ActiveRuntimeResult.Failure(
                code = RuntimeInstallErrorCode.RUNTIME_VERSION_INVALID,
                message = "version must not be blank"
            )
        }

        if (version.contains("..") || version.startsWith("/") || version.contains("\\")) {
            return ActiveRuntimeResult.Failure(
                code = RuntimeInstallErrorCode.RUNTIME_VERSION_INVALID,
                message = "version contains invalid path characters: $version"
            )
        }

        if (!version.matches(VALID_VERSION_PATTERN)) {
            return ActiveRuntimeResult.Failure(
                code = RuntimeInstallErrorCode.RUNTIME_VERSION_INVALID,
                message = "version contains invalid characters: $version"
            )
        }

        return null
    }

    private val VALID_VERSION_PATTERN = Regex("^[a-zA-Z0-9._+\\-]+$")

    companion object {
        private const val MARKER_SCHEMA_VERSION = 1
    }
}
