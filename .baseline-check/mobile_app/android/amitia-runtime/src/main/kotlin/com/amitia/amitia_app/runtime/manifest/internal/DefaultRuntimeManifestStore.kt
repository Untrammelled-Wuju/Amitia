package com.amitia.amitia_app.runtime.manifest.internal

import com.amitia.amitia_app.runtime.manifest.RuntimeManifest
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestError
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestErrorCode
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestResult
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestStore
import java.io.File

internal class DefaultRuntimeManifestStore(
    metadataRoot: String,
) : RuntimeManifestStore {

    private val pathResolver = RuntimeManifestPathResolver(metadataRoot)

    override fun read(): RuntimeManifestResult {
        val manifestFile = File(pathResolver.manifestPath())
        val shaFile = File(pathResolver.manifestShaPath())
        if (!manifestFile.exists()) {
            return RuntimeManifestResult.failure(
                RuntimeManifestError(
                    RuntimeManifestErrorCode.MANIFEST_NOT_FOUND,
                    "runtime manifest not found: ${pathResolver.manifestPath()}"
                )
            )
        }
        val json = try {
            manifestFile.readText(Charsets.UTF_8)
        } catch (e: Exception) {
            return RuntimeManifestResult.failure(
                RuntimeManifestError(
                    RuntimeManifestErrorCode.MANIFEST_READ_FAILED,
                    "failed to read manifest: ${e.message}",
                    cause = e
                )
            )
        }

        if (shaFile.exists()) {
            val shaContent = try {
                shaFile.readText(Charsets.UTF_8).trim()
            } catch (e: Exception) {
                return RuntimeManifestResult.failure(
                    RuntimeManifestError(
                        RuntimeManifestErrorCode.MANIFEST_READ_FAILED,
                        "failed to read manifest sha: ${e.message}",
                        cause = e
                    )
                )
            }
            val actualHash = InstalledFileHasher.sha256String(json)
            val expectedHash = shaContent.substringBefore(' ').trim()
            if (!actualHash.equals(expectedHash, ignoreCase = true)) {
                return RuntimeManifestResult.failure(
                    RuntimeManifestError(
                        RuntimeManifestErrorCode.MANIFEST_HASH_MISMATCH,
                        "manifest hash mismatch: expected=$expectedHash actual=$actualHash"
                    )
                )
            }
        }

        return try {
            val manifest = RuntimeManifestJson.deserialize(json)
            RuntimeManifestResult.success(manifest)
        } catch (e: RuntimeManifestError) {
            RuntimeManifestResult.failure(e)
        } catch (e: RuntimeException) {
            RuntimeManifestResult.failure(
                RuntimeManifestError(
                    RuntimeManifestErrorCode.MANIFEST_INVALID_JSON,
                    "failed to parse manifest: ${e.message}",
                    cause = e
                )
            )
        }
    }

    override fun write(manifest: RuntimeManifest): RuntimeManifestResult {
        val manifestFile = File(pathResolver.manifestPath())
        val shaFile = File(pathResolver.manifestShaPath())
        val manifestTmp = File(pathResolver.manifestTempPath())
        val shaTmp = File(pathResolver.manifestShaTempPath())

        val json = RuntimeManifestJson.serialize(manifest)
        val sha = InstalledFileHasher.sha256String(json)
        val shaLine = sha + "  runtime-manifest.json\n"

        try {
            manifestFile.parentFile?.mkdirs()
            shaFile.parentFile?.mkdirs()

            manifestTmp.writeText(json, Charsets.UTF_8)
            shaTmp.writeText(shaLine, Charsets.UTF_8)

            if (!manifestTmp.renameTo(manifestFile)) {
                if (manifestFile.exists() && !manifestFile.delete()) {
                    return RuntimeManifestResult.failure(
                        RuntimeManifestError(
                            RuntimeManifestErrorCode.MANIFEST_WRITE_FAILED,
                            "failed to replace manifest atomically"
                        )
                    )
                }
                if (!manifestTmp.renameTo(manifestFile)) {
                    return RuntimeManifestResult.failure(
                        RuntimeManifestError(
                            RuntimeManifestErrorCode.MANIFEST_WRITE_FAILED,
                            "failed to write manifest"
                        )
                    )
                }
            }

            if (!shaTmp.renameTo(shaFile)) {
                if (shaFile.exists() && !shaFile.delete()) {
                    return RuntimeManifestResult.failure(
                        RuntimeManifestError(
                            RuntimeManifestErrorCode.MANIFEST_WRITE_FAILED,
                            "failed to replace manifest sha atomically"
                        )
                    )
                }
                if (!shaTmp.renameTo(shaFile)) {
                    return RuntimeManifestResult.failure(
                        RuntimeManifestError(
                            RuntimeManifestErrorCode.MANIFEST_WRITE_FAILED,
                            "failed to write manifest sha"
                        )
                    )
                }
            }

            return RuntimeManifestResult.success(manifest)
        } catch (e: Exception) {
            return RuntimeManifestResult.failure(
                RuntimeManifestError(
                    RuntimeManifestErrorCode.MANIFEST_WRITE_FAILED,
                    "failed to write manifest: ${e.message}",
                    cause = e
                )
            )
        }
    }

    override fun delete(): RuntimeManifestResult {
        val manifestFile = File(pathResolver.manifestPath())
        val shaFile = File(pathResolver.manifestShaPath())
        return try {
            val manifestDeleted = !manifestFile.exists() || manifestFile.delete()
            val shaDeleted = !shaFile.exists() || shaFile.delete()
            if (manifestDeleted && shaDeleted) {
                RuntimeManifestResult.success(RuntimeManifestJson.deserialize("{\"schemaVersion\":1}"))
            } else {
                RuntimeManifestResult.failure(
                    RuntimeManifestError(
                        RuntimeManifestErrorCode.MANIFEST_WRITE_FAILED,
                        "failed to delete manifest files"
                    )
                )
            }
        } catch (e: Exception) {
            RuntimeManifestResult.failure(
                RuntimeManifestError(
                    RuntimeManifestErrorCode.MANIFEST_WRITE_FAILED,
                    "failed to delete manifest: ${e.message}",
                    cause = e
                )
            )
        }
    }
}
