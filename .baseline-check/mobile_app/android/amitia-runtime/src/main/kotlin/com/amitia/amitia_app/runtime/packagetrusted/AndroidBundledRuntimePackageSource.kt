package com.amitia.amitia_app.runtime.packagetrusted

import android.content.Context
import com.amitia.amitia_app.runtime.install.RuntimeHostLayout
import java.io.File
import java.security.MessageDigest
import java.util.UUID

class AndroidBundledRuntimePackageSource(
    private val context: Context,
    private val layout: RuntimeHostLayout,
) : RuntimePackageSource {

    override fun materialize(): RuntimePackageSourceResult {
        val expectedSha = TrustedRuntimePackageSource.PACKAGE_SHA256
        val finalFile = File(layout.packagesRoot, TrustedRuntimePackageSource.FILE_NAME)

        if (finalFile.exists()) {
            val existingSha = computeSha256(finalFile)
            if (existingSha.equals(expectedSha, ignoreCase = true)) {
                return RuntimePackageSourceResult.Ready(
                    TrustedRuntimePackageSource.createReference(finalFile)
                )
            }
            if (!finalFile.delete()) {
                return RuntimePackageSourceResult.Failed(
                    RuntimePackageSourceErrorCode.PACKAGE_MATERIALIZE_FAILED,
                    "Failed to remove stale package cache: ${finalFile.absolutePath}"
                )
            }
        }

        if (!layout.packagesRoot.exists()) {
            if (!layout.packagesRoot.mkdirs()) {
                return RuntimePackageSourceResult.Failed(
                    RuntimePackageSourceErrorCode.PACKAGE_MATERIALIZE_FAILED,
                    "Failed to create packages root: ${layout.packagesRoot.absolutePath}"
                )
            }
        }

        val tempFile = File(
            layout.packagesRoot,
            ".${TrustedRuntimePackageSource.FILE_NAME}.tmp-${UUID.randomUUID()}"
        )

        return try {
            context.applicationContext.assets.open(TrustedRuntimePackageSource.ASSET_PATH).use { input ->
                tempFile.outputStream().use { output ->
                    input.copyTo(output)
                    output.flush()
                    output.fd.sync()
                }
            }

            val tempSha = computeSha256(tempFile)
            if (!tempSha.equals(expectedSha, ignoreCase = true)) {
                return RuntimePackageSourceResult.Failed(
                    RuntimePackageSourceErrorCode.BUNDLED_PACKAGE_HASH_MISMATCH,
                    "Bundled package hash mismatch: expected=$expectedSha actual=$tempSha"
                )
            }

            if (!tempFile.renameTo(finalFile)) {
                return RuntimePackageSourceResult.Failed(
                    RuntimePackageSourceErrorCode.PACKAGE_MATERIALIZE_FAILED,
                    "Failed to move package to final location: ${finalFile.absolutePath}"
                )
            }

            val finalSha = computeSha256(finalFile)
            if (!finalSha.equals(expectedSha, ignoreCase = true)) {
                return RuntimePackageSourceResult.Failed(
                    RuntimePackageSourceErrorCode.BUNDLED_PACKAGE_HASH_MISMATCH,
                    "Final package hash mismatch: expected=$expectedSha actual=$finalSha"
                )
            }

            if (!finalFile.canonicalFile.absolutePath.startsWith(layout.packagesRoot.canonicalFile.absolutePath)) {
                return RuntimePackageSourceResult.Failed(
                    RuntimePackageSourceErrorCode.PACKAGE_CACHE_INVALID,
                    "Package path escaped packages root: ${finalFile.absolutePath}"
                )
            }

            RuntimePackageSourceResult.Ready(
                TrustedRuntimePackageSource.createReference(finalFile)
            )
        } catch (e: java.io.FileNotFoundException) {
            RuntimePackageSourceResult.Failed(
                RuntimePackageSourceErrorCode.BUNDLED_PACKAGE_MISSING,
                "Bundled package asset not found: ${TrustedRuntimePackageSource.ASSET_PATH}"
            )
        } catch (e: Exception) {
            RuntimePackageSourceResult.Failed(
                RuntimePackageSourceErrorCode.PACKAGE_MATERIALIZE_FAILED,
                "Failed to materialize bundled package: ${e.message}"
            )
        } finally {
            if (tempFile.exists()) {
                runCatching { tempFile.delete() }
            }
        }
    }

    private fun computeSha256(file: File): String {
        val digest = MessageDigest.getInstance("SHA-256")
        file.inputStream().use { input ->
            val buffer = ByteArray(8192)
            var read: Int
            while (input.read(buffer).also { read = it } != -1) {
                digest.update(buffer, 0, read)
            }
        }
        return digest.digest().joinToString("") { "%02x".format(it) }
    }
}
