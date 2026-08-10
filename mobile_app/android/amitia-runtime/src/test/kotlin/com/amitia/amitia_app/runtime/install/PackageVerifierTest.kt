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

    private fun createValidPackage(
        baseDir: File,
        filename: String,
        version: String = "1.0.0",
        hostPlatform: String = "android",
        hostAbi: String = "arm64-v8a",
        guestPlatform: String = "linux",
        guestArch: String = "arm64",
    ): File {
        val zipFile = File(baseDir, filename)

        val rootfsContent = "rootfs-content"
        val runtimeContent = "runtime-content"
        val guestLayoutContent = "{\"root\":\"/\"}"
        val mountContractContent = "{\"binds\":[]}"

        val rootfsHash = sha256Hex(rootfsContent)
        val runtimeHash = sha256Hex(runtimeContent)
        val glHash = sha256Hex(guestLayoutContent)
        val mcHash = sha256Hex(mountContractContent)

        val packageIndex = """
            {
                "runtimeVersion": "$version",
                "packageId": "test-pkg-001",
                "target": {
                    "hostPlatform": "$hostPlatform",
                    "hostAbi": "$hostAbi",
                    "runtimeKind": "proot",
                    "guestPlatform": "$guestPlatform",
                    "guestArchitecture": "$guestArch"
                },
                "payloads": [
                    {"role": "rootfs", "path": "payload/rootfs/rootfs.tar.xz", "sha256": "$rootfsHash", "size": 100},
                    {"role": "runtime", "path": "payload/runtime/runtime.tar.xz", "sha256": "$runtimeHash", "size": 200}
                ],
                "metadata": [
                    {"role": "guest-layout", "path": "metadata/guest-layout.json", "sha256": "$glHash", "size": 50},
                    {"role": "mount-contract", "path": "metadata/mount-contract.json", "sha256": "$mcHash", "size": 50},
                    {"role": "sha256sums", "path": "metadata/SHA256SUMS", "sha256": "sums-sha-placeholder", "size": 30}
                ]
            }
        """.trimIndent()

        val componentLock = """
            {
                "runtimeVersion": "$version",
                "packageId": "test-pkg-001",
                "components": [
                    {"id": "backend", "path": "backend/amitia-server", "sha256": "backend-sha-placeholder"}
                ]
            }
        """.trimIndent()

        val sha256sums = """
            $rootfsHash  payload/rootfs/rootfs.tar.xz
            $runtimeHash  payload/runtime/runtime.tar.xz
            $glHash  metadata/guest-layout.json
            $mcHash  metadata/mount-contract.json
        """.trimIndent()

        ZipOutputStream(zipFile.outputStream()).use { zos ->
            val entries = linkedMapOf(
                "metadata/package-index.json" to packageIndex,
                "metadata/component-lock.json" to componentLock,
                "metadata/SHA256SUMS" to sha256sums,
                "payload/rootfs/rootfs.tar.xz" to rootfsContent,
                "payload/runtime/runtime.tar.xz" to runtimeContent,
                "metadata/guest-layout.json" to guestLayoutContent,
                "metadata/mount-contract.json" to mountContractContent,
            )
            for ((path, content) in entries) {
                zos.putNextEntry(ZipEntry(path))
                zos.write(content.toByteArray())
                zos.closeEntry()
            }
        }

        return zipFile
    }

    private fun sha256Hex(content: String): String {
        val bytes = content.toByteArray()
        val digest = java.security.MessageDigest.getInstance("SHA-256")
        val hash = digest.digest(bytes)
        return hash.joinToString("") { "%02x".format(it) }
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
    fun verify_returnsFailure_whenVersionMismatch() {
        val baseDir = tempFolder.newFolder("pkg-version-mismatch")
        val pkgFile = createValidPackage(baseDir, "pkg.zip", version = "1.0.0")

        val verifier = DefaultPackageVerifier()
        val result = verifier.verify(pkgFile, "2.0.0")

        assertTrue(result is PackageVerificationResult.Failure)
        assertEquals(RuntimeInstallErrorCode.PACKAGE_VERSION_MISMATCH, (result as PackageVerificationResult.Failure).code)
    }

    @Test
    fun verify_returnsFailure_whenHostPlatformWrong() {
        val baseDir = tempFolder.newFolder("pkg-platform-wrong")
        val pkgFile = createValidPackage(baseDir, "pkg.zip", hostPlatform = "ios")

        val verifier = DefaultPackageVerifier()
        val result = verifier.verify(pkgFile, null)

        assertTrue(result is PackageVerificationResult.Failure)
        assertEquals(RuntimeInstallErrorCode.PACKAGE_TARGET_MISMATCH, (result as PackageVerificationResult.Failure).code)
    }

    @Test
    fun verify_returnsFailure_whenHostAbiWrong() {
        val baseDir = tempFolder.newFolder("pkg-abi-wrong")
        val pkgFile = createValidPackage(baseDir, "pkg.zip", hostAbi = "x86_64")

        val verifier = DefaultPackageVerifier()
        val result = verifier.verify(pkgFile, null)

        assertTrue(result is PackageVerificationResult.Failure)
        assertEquals(RuntimeInstallErrorCode.PACKAGE_TARGET_MISMATCH, (result as PackageVerificationResult.Failure).code)
    }

    @Test
    fun verify_returnsFailure_whenGuestPlatformWrong() {
        val baseDir = tempFolder.newFolder("pkg-guest-wrong")
        val pkgFile = createValidPackage(baseDir, "pkg.zip", guestPlatform = "windows")

        val verifier = DefaultPackageVerifier()
        val result = verifier.verify(pkgFile, null)

        assertTrue(result is PackageVerificationResult.Failure)
        assertEquals(RuntimeInstallErrorCode.PACKAGE_TARGET_MISMATCH, (result as PackageVerificationResult.Failure).code)
    }

    @Test
    fun verify_returnsFailure_whenGuestArchWrong() {
        val baseDir = tempFolder.newFolder("pkg-arch-wrong")
        val pkgFile = createValidPackage(baseDir, "pkg.zip", guestArch = "amd64")

        val verifier = DefaultPackageVerifier()
        val result = verifier.verify(pkgFile, null)

        assertTrue(result is PackageVerificationResult.Failure)
        assertEquals(RuntimeInstallErrorCode.PACKAGE_TARGET_MISMATCH, (result as PackageVerificationResult.Failure).code)
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
}
