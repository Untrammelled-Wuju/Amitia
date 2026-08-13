package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.proot.ProotArtifact
import com.amitia.amitia_app.runtime.proot.ProotAvailability
import com.amitia.amitia_app.runtime.proot.ProotBinaryLocator
import com.amitia.amitia_app.runtime.proot.ProotErrorCode
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.io.File

class ProotArtifactVerifierTest {

    private fun validSha(): String = "a".repeat(64)

    private fun fakeMetadataLoader(artifact: ProotArtifact): ProotMetadataVerifier {
        return object : ProotMetadataVerifier {
            override fun loadArtifact(): ProotMetadataResult = ProotMetadataResult.Success(artifact)
        }
    }

    @Test
    fun verifier_withNonExecutableFile_returnsNotExecutable() {
        val artifact = ProotArtifact.create("1.0.0", validSha())
        val tempFile = File.createTempFile("proot_test_", ".so")
        tempFile.deleteOnExit()
        tempFile.setReadable(true)
        tempFile.setExecutable(false)
        val locator = ProotBinaryLocator { tempFile }
        val verifier = DefaultProotArtifactVerifier(locator, fakeMetadataLoader(artifact))
        val result = verifier.verify()
        assertTrue(result is ProotAvailability.Unavailable)
        assertEquals(ProotErrorCode.BINARY_NOT_EXECUTABLE, (result as ProotAvailability.Unavailable).errorCode)
        tempFile.delete()
    }

    @Test
    fun verifier_withChecksumMismatch_returnsChecksumMismatch() {
        val artifact = ProotArtifact.create("1.0.0", "b".repeat(64))
        val tempFile = File.createTempFile("proot_test_", ".so")
        tempFile.deleteOnExit()
        tempFile.writeBytes(ByteArray(100) { 0x7F })
        tempFile.setReadable(true)
        tempFile.setExecutable(true)
        val locator = ProotBinaryLocator { tempFile }
        val verifier = DefaultProotArtifactVerifier(locator, fakeMetadataLoader(artifact))
        val result = verifier.verify()
        assertTrue(result is ProotAvailability.Invalid)
        assertEquals(ProotErrorCode.CHECKSUM_MISMATCH, (result as ProotAvailability.Invalid).errorCode)
        tempFile.delete()
    }

    @Test
    fun verifier_withInvalidElf_returnsElfInvalid() {
        val artifact = ProotArtifact.create("1.0.0", validSha().reversed())
        val tempFile = File.createTempFile("proot_test_", ".so")
        tempFile.deleteOnExit()
        tempFile.writeBytes(ByteArray(200) { 0 })
        tempFile.setReadable(true)
        tempFile.setExecutable(true)
        val locator = ProotBinaryLocator { tempFile }
        val verifier = DefaultProotArtifactVerifier(locator, fakeMetadataLoader(artifact))
        val result = verifier.verify()
        assertTrue(result is ProotAvailability.Invalid)
        assertEquals(ProotErrorCode.ELF_INVALID, (result as ProotAvailability.Invalid).errorCode)
        tempFile.delete()
    }

    @Test
    fun verifier_withX86Elf_returnsArchUnsupported() {
        val artifact = ProotArtifact.create("1.0.0", validSha().reversed())
        val tempFile = File.createTempFile("proot_test_", ".so")
        tempFile.deleteOnExit()
        val elf = ByteArray(64)
        elf[0] = 0x7F; elf[1] = 'E'.code.toByte(); elf[2] = 'L'.code.toByte(); elf[3] = 'F'.code.toByte()
        elf[4] = 2; elf[5] = 1
        elf[16] = 2; elf[17] = 0
        elf[18] = 0x3E; elf[19] = 0
        elf[24] = 0x40
        tempFile.writeBytes(elf + ByteArray(200))
        tempFile.setReadable(true)
        tempFile.setExecutable(true)
        val locator = ProotBinaryLocator { tempFile }
        val verifier = DefaultProotArtifactVerifier(locator, fakeMetadataLoader(artifact))
        val result = verifier.verify()
        assertTrue(result is ProotAvailability.Invalid)
        assertEquals(ProotErrorCode.ELF_ARCH_UNSUPPORTED, (result as ProotAvailability.Invalid).errorCode)
        tempFile.delete()
    }

    @Test
    fun verifier_with32BitElf_returnsClassUnsupported() {
        val artifact = ProotArtifact.create("1.0.0", validSha().reversed())
        val tempFile = File.createTempFile("proot_test_", ".so")
        tempFile.deleteOnExit()
        val elf = ByteArray(64)
        elf[0] = 0x7F; elf[1] = 'E'.code.toByte(); elf[2] = 'L'.code.toByte(); elf[3] = 'F'.code.toByte()
        elf[4] = 1; elf[5] = 1
        elf[16] = 2; elf[17] = 0
        elf[18] = 0x3E; elf[19] = 0
        elf[24] = 0x40
        tempFile.writeBytes(elf + ByteArray(200))
        tempFile.setReadable(true)
        tempFile.setExecutable(true)
        val locator = ProotBinaryLocator { tempFile }
        val verifier = DefaultProotArtifactVerifier(locator, fakeMetadataLoader(artifact))
        val result = verifier.verify()
        assertTrue(result is ProotAvailability.Invalid)
        assertEquals(ProotErrorCode.ELF_CLASS_UNSUPPORTED, (result as ProotAvailability.Invalid).errorCode)
        tempFile.delete()
    }

    @Test
    fun verifier_withMissingFile_returnsBinaryNotFound() {
        val artifact = ProotArtifact.create("1.0.0", validSha())
        val locator = ProotBinaryLocator { File("/nonexistent/libamitia_proot.so") }
        val verifier = DefaultProotArtifactVerifier(locator, fakeMetadataLoader(artifact))
        val result = verifier.verify()
        assertTrue(result is ProotAvailability.Unavailable)
        assertEquals(ProotErrorCode.BINARY_NOT_FOUND, (result as ProotAvailability.Unavailable).errorCode)
    }

    @Test
    fun verifier_withDirectory_returnsBinaryNotFile() {
        val artifact = ProotArtifact.create("1.0.0", validSha())
        val tempDir = File(System.getProperty("java.io.tmpdir") ?: "/tmp")
        val locator = ProotBinaryLocator { tempDir }
        val verifier = DefaultProotArtifactVerifier(locator, fakeMetadataLoader(artifact))
        val result = verifier.verify()
        assertTrue(result is ProotAvailability.Unavailable)
        assertEquals(ProotErrorCode.BINARY_NOT_FILE, (result as ProotAvailability.Unavailable).errorCode)
    }

    @Test
    fun verifier_withInvalidMetadata_returnsInvalid() {
        val errorLoader = object : ProotMetadataVerifier {
            override fun loadArtifact(): ProotMetadataResult =
                ProotMetadataResult.Error(com.amitia.amitia_app.runtime.proot.ProotError.of(ProotErrorCode.METADATA_INVALID, "test"))
        }
        val locator = ProotBinaryLocator { File("/nonexistent/libamitia_proot.so") }
        val verifier = DefaultProotArtifactVerifier(locator, errorLoader)
        val result = verifier.verify()
        assertTrue(result is ProotAvailability.Invalid)
        assertEquals(ProotErrorCode.METADATA_INVALID, (result as ProotAvailability.Invalid).errorCode)
    }

    @Test
    fun verifier_withMissingMetadata_returnsMetadataMissing() {
        val locator = ProotBinaryLocator { File("/nonexistent/libamitia_proot.so") }
        val verifier = DefaultProotArtifactVerifier(locator, null)
        val result = verifier.verify()
        assertTrue(result is ProotAvailability.Unavailable)
        assertEquals(ProotErrorCode.METADATA_MISSING, (result as ProotAvailability.Unavailable).errorCode)
    }

    @Test
    fun verifier_withValidArm64Elf_returnsAvailable() {
        val sha = computeSha256(aarch64ElfBytes())
        val artifact = ProotArtifact.create("1.0.0", sha)
        val tempFile = File.createTempFile("proot_valid_", ".so")
        tempFile.deleteOnExit()
        tempFile.writeBytes(aarch64ElfBytes())
        tempFile.setReadable(true)
        tempFile.setExecutable(true)
        val locator = ProotBinaryLocator { tempFile }
        val verifier = DefaultProotArtifactVerifier(locator, fakeMetadataLoader(artifact))
        val result = verifier.verify()
        assertTrue("Expected Available but got: $result", result is ProotAvailability.Available)
        tempFile.delete()
    }

    private fun aarch64ElfBytes(): ByteArray {
        val elf = ByteArray(256)
        elf[0] = 0x7F; elf[1] = 'E'.code.toByte(); elf[2] = 'L'.code.toByte(); elf[3] = 'F'.code.toByte()
        elf[4] = 2; elf[5] = 1; elf[6] = 1
        elf[16] = 2; elf[17] = 0
        elf[18] = 0xB7.toByte(); elf[19] = 0
        elf[24] = 0x40; elf[25] = 0x10
        return elf
    }

    private fun computeSha256(data: ByteArray): String {
        val digest = java.security.MessageDigest.getInstance("SHA-256")
        return digest.digest(data).joinToString("") { "%02x".format(it) }
    }
}
