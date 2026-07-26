package com.amitia.runtime.linux

import kotlinx.serialization.Serializable
import java.io.File
import java.security.MessageDigest
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class RootfsIntegrityChecker @Inject constructor() {

    fun sha256(file: File): String {
        val digest = MessageDigest.getInstance("SHA-256")
        file.inputStream().use { input ->
            val buffer = ByteArray(BUFFER_SIZE)
            while (true) {
                val read = input.read(buffer)
                if (read == -1) break
                digest.update(buffer, 0, read)
            }
        }
        return digest.digest().toHexString()
    }

    fun sha256String(text: String): String {
        val digest = MessageDigest.getInstance("SHA-256")
        return digest.digest(text.toByteArray(Charsets.UTF_8)).toHexString()
    }

    fun buildManifest(rootfsDir: File, files: List<File>): Manifest {
        val entries = files.map { file ->
            val relative = file.relativeTo(rootfsDir).path.replace('\\', '/')
            ManifestEntry(
                path = relative,
                sha256 = sha256(file),
                size = file.length()
            )
        }
        return Manifest(files = entries)
    }

    fun verifyManifest(rootfsDir: File, manifest: Manifest): RootfsIntegrity {
        val missing = mutableListOf<String>()
        val corrupted = mutableListOf<String>()
        @Suppress("SENSELESS_COMPARISON")
        if (manifest == null) {
            return RootfsIntegrity(
                valid = false,
                missingFiles = listOf("manifest"),
                corruptedFiles = emptyList()
            )
        }
        @Suppress("SENSELESS_COMPARISON")
        val files = manifest.files ?: return RootfsIntegrity(
            valid = false,
            missingFiles = listOf("manifest.files"),
            corruptedFiles = emptyList()
        )
        for (entry in files) {
            val file = safeResolve(rootfsDir, entry.path)
            if (file == null) {
                missing.add(entry.path)
                continue
            }
            if (!file.exists()) {
                missing.add(entry.path)
                continue
            }
            val actual = sha256(file)
            if (!actual.equals(entry.sha256, ignoreCase = true)) {
                corrupted.add(entry.path)
            }
        }
        return RootfsIntegrity(
            valid = missing.isEmpty() && corrupted.isEmpty(),
            missingFiles = missing,
            corruptedFiles = corrupted
        )
    }

    private fun safeResolve(base: File, path: String): File? {
        val resolved = File(base, path).canonicalFile
        val baseCanonical = base.canonicalFile
        return if (resolved.path.startsWith(baseCanonical.path)) resolved else null
    }

    private fun ByteArray.toHexString(): String =
        joinToString("") { "%02x".format(it) }

    companion object {
        private const val BUFFER_SIZE = 64 * 1024
    }
}

@Serializable
data class Manifest(
    val files: List<ManifestEntry>
)

@Serializable
data class ManifestEntry(
    val path: String,
    val sha256: String,
    val size: Long
)

data class RootfsIntegrity(
    val valid: Boolean,
    val missingFiles: List<String>,
    val corruptedFiles: List<String>
)
