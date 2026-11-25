package com.amitia.amitia_app.runtime.packagetrusted

import android.content.Context
import com.amitia.amitia_app.runtime.install.RuntimeHostLayout
import java.io.File
import java.security.MessageDigest
import java.util.UUID

private const val COPY_BUFFER_SIZE = 1024 * 1024

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
            val digest = MessageDigest.getInstance("SHA-256")
            context.applicationContext.assets.open(TrustedRuntimePackageSource.ASSET_PATH).use { input ->
                tempFile.outputStream().use { output ->
                    val buffer = ByteArray(COPY_BUFFER_SIZE)
                    var read = input.read(buffer)
                    while (read != -1) {
                        digest.update(buffer, 0, read)
                        output.write(buffer, 0, read)
                        read = input.read(buffer)
                    }
                    output.flush()
                    output.fd.sync()
                }
            }

            val tempSha = digest.digest().joinToString("") { "%02x".format(it) }
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
          e.printStackTrace()

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
