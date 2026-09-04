package com.amitia.amitia_app.runtime.proot

import com.amitia.amitia_app.runtime.proot.internal.DefaultProotArtifactVerifier
import com.amitia.amitia_app.runtime.proot.internal.ProotMetadataResult
import com.amitia.amitia_app.runtime.proot.internal.ProotMetadataVerifier
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

class ProotNativeResolverTest {

    private fun validSha(): String = "a".repeat(64)

    private fun fakeMetadataLoader(artifact: ProotArtifact): ProotMetadataVerifier {
        return object : ProotMetadataVerifier {
            override fun loadArtifact(): ProotMetadataResult = ProotMetadataResult.Success(artifact)
        }
    }

    @Test
    fun artifact_hasFixedName() {
        val artifact = ProotArtifact.create("5.4.0-amitia.1", validSha())
        assertEquals("libamitia_proot.so", artifact.fileName)
    }

    @Test
    fun artifact_hasFixedAbi() {
        val artifact = ProotArtifact.create("5.4.0-amitia.1", validSha())
        assertEquals("arm64-v8a", artifact.abi)
    }

    @Test
    fun artifact_hasFixedArch() {
        val artifact = ProotArtifact.create("5.4.0-amitia.1", validSha())
        assertEquals("aarch64", artifact.arch)
    }

    @Test
    fun artifact_rejectsInvalidSha() {
        try {
            ProotArtifact.create("5.4.0-amitia.1", "invalid-sha-too-short")
            throw AssertionError("should have thrown")
        } catch (_: IllegalArgumentException) {
        }
    }

    @Test
    fun artifact_rejectsShortSha() {
        try {
            ProotArtifact.create("5.4.0-amitia.1", "abcd")
            throw AssertionError("should have thrown")
        } catch (_: IllegalArgumentException) {
        }
    }

    @Test
    fun artifact_shaNormalizedToLowerCase() {
        val sha = "A".repeat(64)
        val artifact = ProotArtifact.create("5.4.0-amitia.1", sha)
        assertEquals(sha.lowercase(), artifact.sha256)
    }

    @Test
    fun artifact_componentIdIsFixed() {
        val artifact = ProotArtifact.create("5.4.0-amitia.1", validSha())
        assertEquals("runtime.proot", artifact.componentId)
    }

    @Test
    fun binaryLocator_isFunctionalInterface() {
        val locator = ProotBinaryLocator { null }
        assertNotNull(locator)
        assertEquals(null, locator.locate())
    }

    @Test
    fun binaryLocator_returningNull_meansNotFound() {
        val locator = ProotBinaryLocator { null }
        val result: File? = locator.locate()
        assertEquals(null, result)
    }

    @Test
    fun verifier_withMissingFile_returnsNotFound() {
        val artifact = ProotArtifact.create("1.0.0", validSha())
        val locator = ProotBinaryLocator { File("/nonexistent/libamitia_proot.so") }
        val verifier = DefaultProotArtifactVerifier(locator, fakeMetadataLoader(artifact))
        val result = verifier.verify()
        assertTrue(result is ProotAvailability.Unavailable)
        assertEquals(ProotErrorCode.BINARY_NOT_FOUND, (result as ProotAvailability.Unavailable).errorCode)
    }

    @Test
    fun verifier_withDirectoryInsteadOfFile_returnsNotFile() {
        val artifact = ProotArtifact.create("1.0.0", validSha())
        val tempDir = File(System.getProperty("java.io.tmpdir") ?: "/tmp")
        val locator = ProotBinaryLocator { tempDir }
        val verifier = DefaultProotArtifactVerifier(locator, fakeMetadataLoader(artifact))
        val result = verifier.verify()
        assertTrue(result is ProotAvailability.Unavailable)
        assertEquals(ProotErrorCode.BINARY_NOT_FILE, (result as ProotAvailability.Unavailable).errorCode)
    }

    @Test
    fun verifier_withoutMetadataLoader_returnsMetadataMissing() {
        val locator = ProotBinaryLocator { File("/nonexistent/libamitia_proot.so") }
        val verifier = DefaultProotArtifactVerifier(locator, null)
        val result = verifier.verify()
        assertTrue(result is ProotAvailability.Unavailable)
        assertEquals(ProotErrorCode.METADATA_MISSING, (result as ProotAvailability.Unavailable).errorCode)
    }

    @Test
    fun availability_available_containsArtifactAndPath() {
        val artifact = ProotArtifact.create("1.0.0", validSha())
        val avail = ProotAvailability.Available(artifact, "/usr/lib/libamitia_proot.so")
        assertEquals(artifact, avail.artifact)
        assertEquals("/usr/lib/libamitia_proot.so", avail.absoluteBinaryPath)
    }

    @Test
    fun availability_unavailable_hasErrorCode() {
        val avail = ProotAvailability.Unavailable(ProotErrorCode.BINARY_NOT_FOUND, "proot.binary.not_found")
        assertEquals(ProotErrorCode.BINARY_NOT_FOUND, avail.errorCode)
        assertEquals("proot.binary.not_found", avail.messageKey)
    }

    @Test
    fun availability_invalid_hasErrorCode() {
        val avail = ProotAvailability.Invalid(ProotErrorCode.CHECKSUM_MISMATCH, "proot.checksum.mismatch")
        assertEquals(ProotErrorCode.CHECKSUM_MISMATCH, avail.errorCode)
        assertEquals("proot.checksum.mismatch", avail.messageKey)
    }

    @Test
    fun errorCodes_containsRequiredCodes() {
        assertTrue(ProotErrorCode.ALL.contains(ProotErrorCode.BINARY_NOT_FOUND))
        assertTrue(ProotErrorCode.ALL.contains(ProotErrorCode.CHECKSUM_MISMATCH))
        assertTrue(ProotErrorCode.ALL.contains(ProotErrorCode.ELF_ARCH_UNSUPPORTED))
        assertTrue(ProotErrorCode.ALL.contains(ProotErrorCode.UNSUPPORTED_ABI))
        assertTrue(ProotErrorCode.ALL.contains(ProotErrorCode.ELF_INVALID))
    }
}
