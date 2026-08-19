package com.amitia.amitia_app.runtime.install.internal

import com.amitia.amitia_app.runtime.install.ComponentLock
import com.amitia.amitia_app.runtime.install.ComponentRef
import com.amitia.amitia_app.runtime.install.GuestLayout
import com.amitia.amitia_app.runtime.install.MountContract
import com.amitia.amitia_app.runtime.install.PackageIndex
import com.amitia.amitia_app.runtime.install.PackageTarget
import com.amitia.amitia_app.runtime.install.PackageVerificationResult
import com.amitia.amitia_app.runtime.install.PackageVerifier
import com.amitia.amitia_app.runtime.install.PayloadRef
import com.amitia.amitia_app.runtime.install.RuntimeInstallErrorCode
import com.amitia.amitia_app.runtime.install.VerifiedPackage
import com.amitia.amitia_app.runtime.proot.MountRole
import com.amitia.amitia_app.runtime.proot.MountSpec
import com.amitia.amitia_app.runtime.proot.ProotBindMount
import java.io.File
import java.security.MessageDigest
import java.util.zip.ZipFile

internal class DefaultPackageVerifier : PackageVerifier {

    companion object {
        const val METADATA_DIR = "metadata"
        const val PACKAGE_INDEX_PATH = "metadata/package-index.json"
        const val COMPONENT_LOCK_PATH = "metadata/component-lock.json"
        const val SHA256SUMS_PATH = "metadata/SHA256SUMS"
        const val ROOTFS_PAYLOAD_PATH = "payload/rootfs/rootfs.tar.xz"
        const val RUNTIME_PAYLOAD_PATH = "payload/runtime/runtime.tar.xz"
        const val VALID_RUNTIME_KIND = "embedded-proot"
        const val SOURCE_REVISION_PATTERN = "^[a-fA-F0-9]{7,40}$"
        const val SHA256_PATTERN = "^[a-fA-F0-9]{64}$"
    }

    override fun verify(packageFile: File, expectedRuntimeVersion: String?): PackageVerificationResult {
        if (!packageFile.exists()) {
            return PackageVerificationResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_NOT_FOUND,
                "package file not found: ${packageFile.absolutePath}"
            )
        }

        val packageSha256 = try {
            computeFileSha256(packageFile)
        } catch (e: Exception) {
            return PackageVerificationResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_READ_FAILED,
                "failed to read package: ${e.message}"
            )
        }

        val zip = try {
            ZipFile(packageFile)
        } catch (e: Exception) {
            return PackageVerificationResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_INVALID,
                "failed to open package zip: ${e.message}"
            )
        }

        try {
            val seenPaths = HashSet<String>()
            val entries = zip.entries()
            while (entries.hasMoreElements()) {
                val entry = entries.nextElement()
                if (!entry.name.startsWith("metadata/") &&
                    !entry.name.startsWith("payload/") &&
                    !entry.name.startsWith("licenses/")) {
                    return PackageVerificationResult.Failure(
                        RuntimeInstallErrorCode.PACKAGE_INVALID,
                        "unknown entry in package: ${entry.name}"
                    )
                }
                if (seenPaths.contains(entry.name)) {
                    return PackageVerificationResult.Failure(
                        RuntimeInstallErrorCode.ARCHIVE_ENTRY_DUPLICATE,
                        "duplicate entry: ${entry.name}"
                    )
                }
                seenPaths.add(entry.name)
            }
        } catch (e: Exception) {
            return PackageVerificationResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_READ_FAILED,
                "failed to read zip entries: ${e.message}"
            )
        }

        val packageIndexText = readZipEntryText(zip, PACKAGE_INDEX_PATH)
            ?: return PackageVerificationResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_INVALID,
                "missing package index: $PACKAGE_INDEX_PATH"
            )

        val packageIndex = try {
            parsePackageIndex(packageIndexText)
        } catch (e: Exception) {
            return PackageVerificationResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_INVALID,
                "invalid package index: ${e.message}"
            )
        }

        if (expectedRuntimeVersion != null && packageIndex.runtimeVersion != expectedRuntimeVersion) {
            return PackageVerificationResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_VERSION_MISMATCH,
                "version mismatch: expected=$expectedRuntimeVersion actual=${packageIndex.runtimeVersion}"
            )
        }

        if (packageIndex.target.hostPlatform != "android") {
            return PackageVerificationResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_TARGET_MISMATCH,
                "host platform not android: ${packageIndex.target.hostPlatform}"
            )
        }
        if (packageIndex.target.hostAbi != "arm64-v8a") {
            return PackageVerificationResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_TARGET_MISMATCH,
                "host ABI not arm64-v8a: ${packageIndex.target.hostAbi}"
            )
        }
        if (packageIndex.target.runtimeKind != VALID_RUNTIME_KIND && packageIndex.target.runtimeKind != "proot") {
            return PackageVerificationResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_TARGET_MISMATCH,
                "runtime kind not $VALID_RUNTIME_KIND: ${packageIndex.target.runtimeKind}"
            )
        }
        if (packageIndex.target.guestPlatform != "linux") {
            return PackageVerificationResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_TARGET_MISMATCH,
                "guest platform not linux: ${packageIndex.target.guestPlatform}"
            )
        }
        if (packageIndex.target.guestArchitecture != "arm64") {
            return PackageVerificationResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_TARGET_MISMATCH,
                "guest architecture not arm64: ${packageIndex.target.guestArchitecture}"
            )
        }

        if (packageIndex.sourceRevision.isEmpty()) {
            return PackageVerificationResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_INVALID,
                "sourceRevision is required"
            )
        }
        if (!packageIndex.sourceRevision.matches(Regex(SOURCE_REVISION_PATTERN))) {
            return PackageVerificationResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_INVALID,
                "sourceRevision must be 7-40 hex chars: ${packageIndex.sourceRevision}"
            )
        }
        if (packageIndex.target.runtimeKind.isNotEmpty() && !packageIndex.target.runtimeKind.matches(Regex("^[a-zA-Z0-9._-]+$"))) {
            return PackageVerificationResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_INVALID,
                "runtimeKind invalid format: ${packageIndex.target.runtimeKind}"
            )
        }

        val componentLockText = readZipEntryText(zip, COMPONENT_LOCK_PATH)
            ?: return PackageVerificationResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_INVALID,
                "missing component lock: $COMPONENT_LOCK_PATH"
            )

        val componentLock = try {
            parseComponentLock(componentLockText, packageIndex.packageId)
        } catch (e: Exception) {
            return PackageVerificationResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_INVALID,
                "invalid component lock: ${e.message}"
            )
        }

        if (componentLock.runtimeVersion != packageIndex.runtimeVersion) {
            return PackageVerificationResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_INVALID,
                "runtime version mismatch between package-index and component-lock"
            )
        }
        if (componentLock.packageId != packageIndex.packageId) {
            return PackageVerificationResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_INVALID,
                "package-id mismatch between package-index and component-lock"
            )
        }

        val sha256sumsText = readZipEntryText(zip, SHA256SUMS_PATH)
            ?: return PackageVerificationResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_INVALID,
                "missing SHA256SUMS: $SHA256SUMS_PATH"
            )

        val sha256sums = parseSha256sums(sha256sumsText)

        if (!verifySha256sum(zip, packageIndex.rootfsPayload, sha256sums)) {
            return PackageVerificationResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_HASH_MISMATCH,
                "rootfs payload hash mismatch"
            )
        }
        if (!verifySha256sum(zip, packageIndex.runtimePayload, sha256sums)) {
            return PackageVerificationResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_HASH_MISMATCH,
                "runtime payload hash mismatch"
            )
        }
        if (!verifySha256sum(zip, packageIndex.guestLayout, sha256sums)) {
            return PackageVerificationResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_HASH_MISMATCH,
                "guest-layout hash mismatch"
            )
        }
        if (!verifySha256sum(zip, packageIndex.mountContract, sha256sums)) {
            return PackageVerificationResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_HASH_MISMATCH,
                "mount-contract hash mismatch"
            )
        }

        val tempDir = createTempDirForExtraction(packageFile)
        try {
            val extractedRootfs = extractZipEntryToFile(zip, packageIndex.rootfsPayload.path, tempDir)
                ?: return PackageVerificationResult.Failure(
                    RuntimeInstallErrorCode.PACKAGE_READ_FAILED,
                    "failed to extract rootfs payload"
                )
            val extractedRuntime = extractZipEntryToFile(zip, packageIndex.runtimePayload.path, tempDir)
                ?: return PackageVerificationResult.Failure(
                    RuntimeInstallErrorCode.PACKAGE_READ_FAILED,
                    "failed to extract runtime payload"
                )
            val extractedSha256sums = extractZipEntryToFile(zip, packageIndex.sha256sums.path, tempDir)
                ?: return PackageVerificationResult.Failure(
                    RuntimeInstallErrorCode.PACKAGE_READ_FAILED,
                    "failed to extract SHA256SUMS"
                )
            val metadataDir = extractedSha256sums.parentFile

            val guestLayout = parseGuestLayoutFromZip(zip, packageIndex.guestLayout.path)
                ?: return PackageVerificationResult.Failure(
                    RuntimeInstallErrorCode.PACKAGE_INVALID,
                    "failed to parse guest-layout.json"
                )
            val mountContract = parseMountContractFromZip(zip, packageIndex.mountContract.path)
                ?: return PackageVerificationResult.Failure(
                    RuntimeInstallErrorCode.PACKAGE_INVALID,
                    "failed to parse mount-contract.json"
                )

            val verified = VerifiedPackage(
                packageFile = packageFile,
                packageSha256 = packageSha256,
                packageIndex = packageIndex,
                componentLock = componentLock,
                guestLayout = guestLayout,
                mountContract = mountContract,
                rootfsPayloadFile = extractedRootfs,
                runtimePayloadFile = extractedRuntime,
                sha256sumsFile = extractedSha256sums,
                metadataDir = metadataDir,
            )

            return PackageVerificationResult.Success(verified)
        } catch (e: Exception) {
            tempDir.deleteRecursively()
            return PackageVerificationResult.Failure(
                RuntimeInstallErrorCode.PACKAGE_READ_FAILED,
                "extraction failed: ${e.message}"
            )
        } finally {
            try {
                zip.close()
            } catch (_: Exception) {
            }
        }
    }

    private fun computeFileSha256(file: File): String {
        val digest = MessageDigest.getInstance("SHA-256")
        file.inputStream().use { input ->
            val buffer = ByteArray(8192)
            while (true) {
                val n = input.read(buffer)
                if (n <= 0) break
                digest.update(buffer, 0, n)
            }
        }
        return digest.digest().toLowerHex()
    }

    private fun ByteArray.toLowerHex(): String {
        val sb = StringBuilder(size * 2)
        for (b in this) {
            val v = b.toInt() and 0xFF
            sb.append(HEX[v ushr 4])
            sb.append(HEX[v and 0x0F])
        }
        return sb.toString()
    }

    private val HEX = "0123456789abcdef".toCharArray()

    private fun readZipEntryText(zip: ZipFile, entryPath: String): String? {
        val entry = zip.getEntry(entryPath) ?: return null
        return try {
            zip.getInputStream(entry).bufferedReader(Charsets.UTF_8).use { it.readText() }
        } catch (_: Exception) {
            null
        }
    }

    private fun extractZipEntryToFile(zip: ZipFile, entryPath: String, targetDir: File): File? {
        val entry = zip.getEntry(entryPath) ?: return null
        val targetFile = File(targetDir, entryPath.substringAfterLast('/'))
        return try {
            targetFile.parentFile?.mkdirs()
            zip.getInputStream(entry).use { input ->
                targetFile.outputStream().use { output ->
                    input.copyTo(output)
                }
            }
            targetFile
        } catch (_: Exception) {
            null
        }
    }

    private fun parsePackageIndex(text: String): PackageIndex {
        val runtimeVersion = extractJsonString(text, "runtimeVersion")
        val packageId = extractJsonString(text, "packageId")
        val sourceRevision = extractJsonStringOrNull(text, "sourceRevision")
            ?: extractJsonString(text, "sourceCommit")

        val targetJson = extractJsonObject(text, "target")
        val hostPlatform = extractJsonString(targetJson, "hostPlatform")
        val hostAbi = extractJsonString(targetJson, "hostAbi")
        val runtimeKind = extractJsonString(targetJson, "runtimeKind")
        val guestPlatform = extractJsonString(targetJson, "guestPlatform")
        val guestArchitecture = extractJsonString(targetJson, "guestArchitecture")

        var rootfsPayload: PayloadRef? = null
        var runtimePayload: PayloadRef? = null
        if (Regex("\"payloads\"\\s*:\\s*\\[").containsMatchIn(text)) {
            for (item in extractJsonArray(text, "payloads")) {
                val role = extractJsonString(item, "role")
                val path = extractJsonString(item, "path")
                val sha = extractJsonString(item, "sha256")
                val size = extractJsonNumber(item, "size")
                when (role) {
                    "rootfs" -> rootfsPayload = PayloadRef(path, sha, size)
                    "runtime" -> runtimePayload = PayloadRef(path, sha, size)
                }
            }
        } else {
            val payloads = extractJsonObject(text, "payloads")
            val rootfs = extractJsonObject(payloads, "rootfs")
            val runtime = extractJsonObject(payloads, "runtime")
            rootfsPayload = PayloadRef(
                extractJsonString(rootfs, "path"),
                extractJsonString(rootfs, "sha256"),
                extractJsonNumber(rootfs, "size"),
            )
            runtimePayload = PayloadRef(
                extractJsonString(runtime, "path"),
                extractJsonString(runtime, "sha256"),
                extractJsonNumber(runtime, "size"),
            )
        }

        if (rootfsPayload == null) throw IllegalArgumentException("no rootfs payload found")
        if (runtimePayload == null) throw IllegalArgumentException("no runtime payload found")

        var guestLayout: PayloadRef? = null
        var mountContract: PayloadRef? = null
        var sha256sums: PayloadRef? = null
        if (Regex("\"metadata\"\\s*:\\s*\\[").containsMatchIn(text)) {
            for (item in extractJsonArray(text, "metadata")) {
                val role = extractJsonString(item, "role")
                val path = extractJsonString(item, "path")
                val sha = extractJsonString(item, "sha256")
                val size = extractJsonNumber(item, "size")
                when (role) {
                    "guest-layout" -> guestLayout = PayloadRef(path, sha, size)
                    "mount-contract" -> mountContract = PayloadRef(path, sha, size)
                    "sha256sums" -> sha256sums = PayloadRef(path, sha, size)
                }
            }
        } else {
            val metadata = extractJsonObject(text, "metadata")
            guestLayout = PayloadRef(extractJsonString(metadata, "guestLayout"), "", 0)
            mountContract = PayloadRef(extractJsonString(metadata, "mountContract"), "", 0)
            sha256sums = PayloadRef(SHA256SUMS_PATH, "", 0)
        }

        if (guestLayout == null) throw IllegalArgumentException("no guest-layout metadata found")
        if (mountContract == null) throw IllegalArgumentException("no mount-contract metadata found")
        if (sha256sums == null) throw IllegalArgumentException("no sha256sums metadata found")

        return PackageIndex(
            runtimeVersion = runtimeVersion,
            packageId = packageId,
            sourceRevision = sourceRevision,
            target = PackageTarget(
                hostPlatform = hostPlatform,
                hostAbi = hostAbi,
                runtimeKind = runtimeKind,
                guestPlatform = guestPlatform,
                guestArchitecture = guestArchitecture,
            ),
            guestLayout = guestLayout,
            mountContract = mountContract,
            rootfsPayload = rootfsPayload,
            runtimePayload = runtimePayload,
            sha256sums = sha256sums,
            licenses = null,
        )
    }

    private fun parseComponentLock(text: String, packageIdFallback: String): ComponentLock {
        val runtimeVersion = extractJsonString(text, "runtimeVersion")
        val packageId = extractJsonStringOrNull(text, "packageId") ?: packageIdFallback
        val components = parseComponentArray(text)
        return ComponentLock(
            runtimeVersion = runtimeVersion,
            packageId = packageId,
            components = components,
        )
    }

    private fun parseComponentArray(text: String): List<ComponentRef> {
        val result = mutableListOf<ComponentRef>()
        val componentsArrayContent = extractJsonArrayContent(text, "components") ?: return result
        val objectPattern = Regex("\\{[^{}]+\\}")
        for (objMatch in objectPattern.findAll(componentsArrayContent)) {
            val obj = objMatch.value
            result.add(
                ComponentRef(
                    id = extractJsonString(obj, "id"),
                    version = extractJsonStringOrNull(obj, "version"),
                    architecture = extractJsonStringOrNull(obj, "architecture"),
                    path = extractJsonString(obj, "path"),
                    sha256 = extractJsonString(obj, "sha256"),
                )
            )
        }
        return result
    }

    private fun parseSha256sums(text: String): Map<String, String> {
        val result = mutableMapOf<String, String>()
        for (line in text.lines()) {
            val trimmed = line.trim()
            if (trimmed.isEmpty()) continue
            val parts = trimmed.split(Regex("\\s+"), limit = 2)
            if (parts.size == 2) {
                result[parts[1].trim()] = parts[0].trim()
            }
        }
        return result
    }

    private fun verifySha256sum(zip: ZipFile, ref: PayloadRef, sha256sums: Map<String, String>): Boolean {
        val expectedHash = sha256sums[ref.path]
            ?: sha256sums[ref.path.substringBeforeLast('/')]
            ?: return false
        val entry = zip.getEntry(ref.path) ?: return false
        val actualHash = try {
            val digest = MessageDigest.getInstance("SHA-256")
            zip.getInputStream(entry).use { input ->
                val buffer = ByteArray(8192)
                while (true) {
                    val n = input.read(buffer)
                    if (n <= 0) break
                    digest.update(buffer, 0, n)
                }
            }
            digest.digest().toLowerHex()
        } catch (_: Exception) {
            return false
        }
        return actualHash.equals(expectedHash, ignoreCase = true)
    }

    private fun verifySha256sumText(textPath: String, expectedSha: String, sha256sums: Map<String, String>): Boolean {
        val expectedHash = sha256sums[textPath] ?: return false
        return expectedHash.equals(expectedSha, ignoreCase = true)
    }

    private fun createTempDirForExtraction(packageFile: File): File {
        val baseDir = packageFile.parentFile ?: File(System.getProperty("java.io.tmpdir"))
        val tempDir = File(baseDir, ".pkg-verify-" + System.nanoTime())
        tempDir.mkdirs()
        return tempDir
    }

    private fun extractJsonString(json: String, key: String): String {
        val pattern = Regex("\"$key\"\\s*:\\s*\"([^\"]+)\"")
        val match = pattern.find(json) ?: throw IllegalArgumentException("missing key: $key")
        return match.groupValues[1]
    }

    private fun extractJsonStringOrNull(json: String, key: String): String? {
        val pattern = Regex("\"$key\"\\s*:\\s*\"([^\"]+)\"")
        return pattern.find(json)?.groupValues?.get(1)
    }

    private fun extractJsonNumber(json: String, key: String): Long {
        val pattern = Regex("\"$key\"\\s*:\\s*(\\d+)")
        val match = pattern.find(json) ?: throw IllegalArgumentException("missing number key: $key")
        return match.groupValues[1].toLongOrNull() ?: throw IllegalArgumentException("invalid number for key: $key")
    }

    private fun extractJsonObject(text: String, key: String): String {
        val pattern = Regex("\"$key\"\\s*:\\s*\\{")
        val startMatch = pattern.find(text) ?: throw IllegalArgumentException("missing object: $key")
        var depth = 0
        var end = -1
        for (i in startMatch.range.last until text.length) {
            when (text[i]) {
                '{' -> depth++
                '}' -> {
                    depth--
                    if (depth == 0) {
                        end = i + 1
                        break
                    }
                }
            }
        }
        if (end == -1) throw IllegalArgumentException("unterminated object: $key")
        return text.substring(startMatch.range.last, end)
    }

    private fun extractJsonArray(text: String, key: String): List<String> {
        val pattern = Regex("\"$key\"\\s*:\\s*\\[")
        val startMatch = pattern.find(text) ?: throw IllegalArgumentException("missing array: $key")
        var depth = 0
        var end = -1
        for (i in startMatch.range.last until text.length) {
            when (text[i]) {
                '[' -> depth++
                ']' -> {
                    depth--
                    if (depth == 0) {
                        end = i
                        break
                    }
                }
            }
        }
        if (end == -1) throw IllegalArgumentException("unterminated array: $key")
        val arrayContent = text.substring(startMatch.range.last, end)
        return parseArrayObjects(arrayContent)
    }

    private fun extractJsonArrayContent(text: String, key: String): String? {
        val pattern = Regex("\"$key\"\\s*:\\s*\\[")
        val startMatch = pattern.find(text) ?: return null
        var depth = 0
        var end = -1
        for (i in startMatch.range.last until text.length) {
            when (text[i]) {
                '[' -> depth++
                ']' -> {
                    depth--
                    if (depth == 0) {
                        end = i + 1
                        break
                    }
                }
            }
        }
        if (end == -1) return null
        return text.substring(startMatch.range.last, end)
    }

    private fun parseArrayObjects(content: String): List<String> {
        val result = mutableListOf<String>()
        var depth = 0
        var start = -1
        for (i in content.indices) {
            when (content[i]) {
                '{' -> {
                    if (depth == 0) start = i
                    depth++
                }
                '}' -> {
                    depth--
                    if (depth == 0 && start != -1) {
                        result.add(content.substring(start, i + 1))
                        start = -1
                    }
                }
            }
        }
        return result
    }

    private fun parseGuestLayoutFromZip(zip: ZipFile, entryPath: String): GuestLayout? {
        val text = readZipEntryText(zip, entryPath) ?: return null
        return try {
            val root = extractJsonStringOrNull(text, "root")
                ?: extractJsonString(extractJsonObject(text, "paths"), "runtimeRoot")
            val directories = mutableListOf<String>()
            val arrayContent = extractJsonArrayContent(text, "directories") ?: "[]"
            val dirPattern = Regex("\"([^\"]+)\"")
            for (match in dirPattern.findAll(arrayContent)) {
                val dir = match.groupValues[1]
                if (dir.startsWith("/")) {
                    directories.add(dir)
                }
            }
            GuestLayout(root = root, directories = directories)
        } catch (_: Exception) {
            null
        }
    }

    private fun parseMountContractFromZip(zip: ZipFile, entryPath: String): MountContract? {
        val text = readZipEntryText(zip, entryPath) ?: return null
        return try {
            MountContract(binds = emptyList())
        } catch (_: Exception) {
            null
        }
    }

    private fun extractJsonBoolean(json: String, key: String): Boolean {
        val pattern = Regex("\"$key\"\\s*:\\s*(true|false)")
        val match = pattern.find(json) ?: return false
        return match.groupValues[1] == "true"
    }
}
