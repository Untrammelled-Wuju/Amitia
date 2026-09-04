package com.amitia.amitia_app.runtime.install

import com.amitia.amitia_app.runtime.install.internal.DefaultPackageVerifier
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.rules.TemporaryFolder
import java.io.File
import java.util.zip.ZipEntry
import java.util.zip.ZipOutputStream

class PackageVerifierTest {

    @get:Rule
    val tempFolder = TemporaryFolder()

    private fun sha256Hex(content: String): String {
        val bytes = content.toByteArray()
        val digest = java.security.MessageDigest.getInstance("SHA-256")
        val hash = digest.digest(bytes)
        return hash.joinToString("") { "%02x".format(it) }
    }

    private fun createMinimalValidPackage(baseDir: File, filename: String): File {
        val zipFile = File(baseDir, filename)

        val rootfsContent = "rootfs-payload-content"
        val runtimeContent = "runtime-payload-content"
        val guestLayoutContent = "{\"root\":\"/\"}"
        val mountContractContent = "{\"binds\":[]}"

        val rootfsHash = sha256Hex(rootfsContent)
        val runtimeHash = sha256Hex(runtimeContent)
        val glHash = sha256Hex(guestLayoutContent)
        val mcHash = sha256Hex(mountContractContent)

        val packageIndex = """{"runtimeVersion":"1.0.0","packageId":"test-pkg","target":{"hostPlatform":"android","hostAbi":"arm64-v8a","runtimeKind":"proot","guestPlatform":"linux","guestArchitecture":"arm64"},"payloads":[{"role":"rootfs","path":"payload/rootfs/rootfs.tar.xz","sha256":"$rootfsHash","size":"100"},{"role":"runtime","path":"payload/runtime/runtime.tar.xz","sha256":"$runtimeHash","size":"200"}],"metadata":[{"role":"guest-layout","path":"metadata/guest-layout.json","sha256":"$glHash","size":"50"},{"role":"mount-contract","path":"metadata/mount-contract.json","sha256":"$mcHash","size":"50"},{"role":"sha256sums","path":"metadata/SHA256SUMS","sha256":"sums-hash","size":"30"}]}"""
        val componentLock = """{"runtimeVersion":"1.0.0","packageId":"test-pkg","components":[{"id":"backend","path":"backend/amitia-server","sha256":"backend-sha"}]}"""
        val sha256sums = "$rootfsHash  payload/rootfs/rootfs.tar.xz\n$runtimeHash  payload/runtime/runtime.tar.xz\n$glHash  metadata/guest-layout.json\n$mcHash  metadata/mount-contract.json"
        val idxHash = sha256Hex(packageIndex)
        val clHash = sha256Hex(componentLock)
        val sumsHash2 = sha256Hex(sha256sums)
        val finalSums = "$rootfsHash  payload/rootfs/rootfs.tar.xz\n$runtimeHash  payload/runtime/runtime.tar.xz\n$glHash  metadata/guest-layout.json\n$mcHash  metadata/mount-contract.json\n$idxHash  metadata/package-index.json\n$clHash  metadata/component-lock.json\n$sumsHash2  metadata/SHA256SUMS"

        ZipOutputStream(zipFile.outputStream()).use { zos ->
            zos.putNextEntry(ZipEntry("metadata/package-index.json"))
            zos.write(packageIndex.toByteArray())
            zos.closeEntry()
            zos.putNextEntry(ZipEntry("metadata/component-lock.json"))
            zos.write(componentLock.toByteArray())
            zos.closeEntry()
            zos.putNextEntry(ZipEntry("metadata/SHA256SUMS"))
            zos.write(finalSums.toByteArray())
            zos.closeEntry()
            zos.putNextEntry(ZipEntry("payload/rootfs/rootfs.tar.xz"))
            zos.write(rootfsContent.toByteArray())
            zos.closeEntry()
            zos.putNextEntry(ZipEntry("payload/runtime/runtime.tar.xz"))
            zos.write(runtimeContent.toByteArray())
            zos.closeEntry()
            zos.putNextEntry(ZipEntry("metadata/guest-layout.json"))
            zos.write(guestLayoutContent.toByteArray())
            zos.closeEntry()
            zos.putNextEntry(ZipEntry("metadata/mount-contract.json"))
            zos.write(mountContractContent.toByteArray())
            zos.closeEntry()
        }

        return zipFile
    }

    @Test
    fun verify_returnsFailure_whenPackageNotFound() {
        val baseDir = tempFolder.newFolder("pkg-not-found")
        val missingFile = File(baseDir, "missing.zip")

        val verifier = DefaultPackageVerifier()
        val result = verifier.verify(missingFile, null)

        assertTrue(result is PackageVerificationResult.Failure)
        assertEquals(RuntimeInstallErrorCode.PACKAGE_NOT_FOUND, (result as PackageVerificationResult.Failure).code)
    }

    @Test
    fun verify_returnsFailure_whenPackageInvalid() {
        val baseDir = tempFolder.newFolder("pkg-invalid")
        val invalidFile = File(baseDir, "invalid.zip")
        invalidFile.writeText("not a zip file")

        val verifier = DefaultPackageVerifier()
        val result = verifier.verify(invalidFile, null)

        assertTrue(result is PackageVerificationResult.Failure)
        assertEquals(RuntimeInstallErrorCode.PACKAGE_INVALID, (result as PackageVerificationResult.Failure).code)
    }

    @Test
    fun verify_returnsFailure_whenMetadataPackageIndexMissing() {
        val baseDir = tempFolder.newFolder("pkg-no-index")
        val zipFile = File(baseDir, "pkg.zip")

        ZipOutputStream(zipFile.outputStream()).use { zos ->
            zos.putNextEntry(ZipEntry("payload/runtime/runtime.tar.xz"))
            zos.write("content".toByteArray())
            zos.closeEntry()
        }

        val verifier = DefaultPackageVerifier()
        val result = verifier.verify(zipFile, null)

        assertTrue(result is PackageVerificationResult.Failure)
        assertEquals(RuntimeInstallErrorCode.PACKAGE_INVALID, (result as PackageVerificationResult.Failure).code)
    }

    @Test
    fun verify_returnsFailure_whenUnknownEntryPresent() {
        val baseDir = tempFolder.newFolder("pkg-unknown-entry")
        val zipFile = File(baseDir, "pkg.zip")

        ZipOutputStream(zipFile.outputStream()).use { zos ->
            zos.putNextEntry(ZipEntry("secret/backdoor.sh"))
            zos.write("evil".toByteArray())
            zos.closeEntry()
        }

        val verifier = DefaultPackageVerifier()
        val result = verifier.verify(zipFile, null)

        assertTrue(result is PackageVerificationResult.Failure)
        assertEquals(RuntimeInstallErrorCode.PACKAGE_INVALID, (result as PackageVerificationResult.Failure).code)
    }

    @Test
    fun verify_returnsFailure_whenChecksumMissing() {
        val baseDir = tempFolder.newFolder("pkg-no-checksums")
        val zipFile = File(baseDir, "pkg.zip")

        val packageIndex = """{"runtimeVersion":"1.0.0","packageId":"test-pkg","target":{"hostPlatform":"android","hostAbi":"arm64-v8a","runtimeKind":"proot","guestPlatform":"linux","guestArchitecture":"arm64"},"payloads":[{"role":"rootfs","path":"payload/rootfs/rootfs.tar.xz","sha256":"hash1","size":100},{"role":"runtime","path":"payload/runtime/runtime.tar.xz","sha256":"hash2","size":200}],"metadata":[{"role":"guest-layout","path":"metadata/guest-layout.json","sha256":"glhash","size":50},{"role":"mount-contract","path":"metadata/mount-contract.json","sha256":"mchash","size":50},{"role":"sha256sums","path":"metadata/SHA256SUMS","sha256":"shash","size":30}]}"""

        ZipOutputStream(zipFile.outputStream()).use { zos ->
            zos.putNextEntry(ZipEntry("metadata/package-index.json"))
            zos.write(packageIndex.toByteArray())
            zos.closeEntry()
            zos.putNextEntry(ZipEntry("metadata/component-lock.json"))
            zos.write("{}".toByteArray())
            zos.closeEntry()
            zos.putNextEntry(ZipEntry("payload/rootfs/rootfs.tar.xz"))
            zos.write("rootfs".toByteArray())
            zos.closeEntry()
            zos.putNextEntry(ZipEntry("payload/runtime/runtime.tar.xz"))
            zos.write("runtime".toByteArray())
            zos.closeEntry()
            zos.putNextEntry(ZipEntry("metadata/guest-layout.json"))
            zos.write("gl".toByteArray())
            zos.closeEntry()
            zos.putNextEntry(ZipEntry("metadata/mount-contract.json"))
            zos.write("mc".toByteArray())
            zos.closeEntry()
        }

        val verifier = DefaultPackageVerifier()
        val result = verifier.verify(zipFile, null)

        assertTrue(result is PackageVerificationResult.Failure)
        assertEquals(RuntimeInstallErrorCode.PACKAGE_INVALID, (result as PackageVerificationResult.Failure).code)
    }

    @Test
    fun verify_succeedsWithValidPackage() {
        val baseDir = tempFolder.newFolder("pkg-valid")
        val pkgFile = createMinimalValidPackage(baseDir, "valid.zip")

        val verifier = DefaultPackageVerifier()
        val result = verifier.verify(pkgFile, null)

        assertTrue("Expected Success but got: $result", result is PackageVerificationResult.Success)
    }

    @Test
    fun verify_returnsVersionMismatch_whenExpectedVersionDiffers() {
        val baseDir = tempFolder.newFolder("pkg-version-check")
        val pkgFile = createMinimalValidPackage(baseDir, "pkg.zip")

        val verifier = DefaultPackageVerifier()
        val result = verifier.verify(pkgFile, "2.0.0")

        assertTrue(result is PackageVerificationResult.Failure)
        assertEquals(RuntimeInstallErrorCode.PACKAGE_VERSION_MISMATCH, (result as PackageVerificationResult.Failure).code)
    }

    @Test
    fun verify_returnsSuccess_whenExpectedVersionMatches() {
        val baseDir = tempFolder.newFolder("pkg-version-match")
        val pkgFile = createMinimalValidPackage(baseDir, "pkg.zip")

        val verifier = DefaultPackageVerifier()
        val result = verifier.verify(pkgFile, "1.0.0")

        assertTrue("Expected Success but got: $result", result is PackageVerificationResult.Success)
    }
}
