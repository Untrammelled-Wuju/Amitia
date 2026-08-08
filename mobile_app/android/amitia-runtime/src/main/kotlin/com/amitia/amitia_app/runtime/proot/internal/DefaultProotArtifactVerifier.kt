package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.proot.ProotAvailability
import com.amitia.amitia_app.runtime.proot.ProotBinaryLocator
import com.amitia.amitia_app.runtime.proot.ProotError
import com.amitia.amitia_app.runtime.proot.ProotErrorCode
import java.io.File
import java.security.MessageDigest

internal fun interface ProotArtifactVerifier {
    fun verify(): ProotAvailability
}

internal class DefaultProotArtifactVerifier(private val locator: ProotBinaryLocator, private val metadataLoader: ProotMetadataVerifier? = null) : ProotArtifactVerifier {
    override fun verify(): ProotAvailability {
        val metadataResult = metadataLoader?.loadArtifact()
        if (metadataResult is ProotMetadataResult.Error) return ProotAvailability.Invalid(metadataResult.error.code, "proot.metadata.invalid")
        val artifact = (metadataResult as? ProotMetadataResult.Success)?.artifact ?: return ProotAvailability.Unavailable(ProotErrorCode.METADATA_MISSING, "proot.metadata.missing")
        val file = locator.locate() ?: return ProotAvailability.Unavailable(ProotErrorCode.BINARY_NOT_FOUND, "proot.binary.not_found")
        if (!file.exists()) return ProotAvailability.Unavailable(ProotErrorCode.BINARY_NOT_FOUND, "proot.binary.not_found")
        if (!file.isFile) return ProotAvailability.Unavailable(ProotErrorCode.BINARY_NOT_FILE, "proot.binary.not_file")
        if (!file.canRead()) return ProotAvailability.Unavailable(ProotErrorCode.BINARY_NOT_READABLE, "proot.binary.not_readable")
        val elfError = ProotElfInspector().inspect(file)
        if (elfError != null) return ProotAvailability.Invalid(elfError.code, "proot.elf.${elfError.code.name.lowercase()}")
        if (!verifyChecksum(file, artifact.sha256)) return ProotAvailability.Invalid(ProotErrorCode.CHECKSUM_MISMATCH, "proot.checksum.mismatch")
        return ProotAvailability.Available(artifact, file.absolutePath)
    }
    private fun verifyChecksum(file: File, expected: String): Boolean = try {
        val digest = MessageDigest.getInstance("SHA-256")
        file.inputStream().use { fis -> val buf = ByteArray(8192); while (true) { val r = fis.read(buf); if (r == -1) break; digest.update(buf, 0, r) } }
        digest.digest().joinToString("") { "%02x".format(it) } == expected
    } catch (e: Exception) { false }
}
internal sealed class ProotMetadataResult {
    data class Success(val artifact: com.amitia.amitia_app.runtime.proot.ProotArtifact) : ProotMetadataResult()
    data class Error(val error: ProotError) : ProotMetadataResult()
}
internal interface ProotMetadataVerifier { fun loadArtifact(): ProotMetadataResult }