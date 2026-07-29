package com.amitia.runtime.extension.security

class EntryValidator(private val policy: ArchivePolicy) {

    private val forbiddenExtensions = setOf(
        ".exe", ".com", ".bat", ".cmd",
        ".msi", ".dll", ".sys", ".scr",
        ".pif", ".cpl", ".so", ".dylib",
        ".app", ".class", ".jar",
        ".apk", ".deb", ".rpm"
    )

    private val nestedArchiveExtensions = setOf(
        ".zip", ".rar", ".7z", ".tar",
        ".gz", ".tgz", ".bz2", ".xz"
    )

    private val executableMagicBytes = listOf(
        byteArrayOf('M'.code.toByte(), 'Z'.code.toByte()),
        byteArrayOf(0x7f, 'E'.code.toByte(), 'L'.code.toByte(), 'F'.code.toByte()),
        byteArrayOf(0xca.toByte(), 0xfe.toByte(), 0xba.toByte(), 0xbe.toByte()),
        byteArrayOf(0xcf.toByte(), 0xfa.toByte(), 0xed.toByte(), 0xfe.toByte()),
        byteArrayOf(0xce.toByte(), 0xfa.toByte(), 0xed.toByte(), 0xfe.toByte())
    )

    private val archiveMagicBytes = listOf(
        byteArrayOf('P'.code.toByte(), 'K'.code.toByte(), 0x03, 0x04),
        byteArrayOf('P'.code.toByte(), 'K'.code.toByte(), 0x05, 0x06),
        byteArrayOf(0x1f, 0x8b.toByte()),
        byteArrayOf('B'.code.toByte(), 'Z'.code.toByte(), 'h'.code.toByte()),
        byteArrayOf(0xfd.toByte(), '7'.code.toByte(), 'z'.code.toByte(), 'X'.code.toByte(), 'Z'.code.toByte(), 0x00),
        byteArrayOf('R'.code.toByte(), 'a'.code.toByte(), 'r'.code.toByte(), '!'.code.toByte(), 0x1a, 0x07)
    )

    private val secretIndicators = listOf(
        "-----BEGIN RSA PRIVATE KEY-----",
        "-----BEGIN EC PRIVATE KEY-----",
        "-----BEGIN PRIVATE KEY-----",
        "-----BEGIN CERTIFICATE-----",
        "ghp_",
        "gho_",
        "ghu_",
        "ghs_",
        "ghr_",
        "sk-"
    )

    data class EntryValidationResult(
        val passed: Boolean,
        val warnings: List<String> = emptyList(),
        val errors: List<String> = emptyList()
    )

    fun validate(path: String, content: ByteArray): EntryValidationResult {
        val errors = mutableListOf<String>()
        val warnings = mutableListOf<String>()

        val ext = getFileExtension(path).lowercase()

        if (forbiddenExtensions.contains(ext)) {
            errors.add("forbidden file extension: $ext")
        }

        if (nestedArchiveExtensions.contains(ext) && !policy.allowNestedArchive) {
            errors.add("nested archive not allowed: $ext")
        }

        if (ext == ".wasm" && content.size >= 4 &&
            content[0] == 0x00.toByte() && content[1] == 'a'.code.toByte() &&
            content[2] == 's'.code.toByte() && content[3] == 'm'.code.toByte()
        ) {
            warnings.add("WASM binary detected: $path")
        }

        for (magic in executableMagicBytes) {
            if (content.size >= magic.size && content.copyOfRange(0, magic.size).contentEquals(magic)) {
                if (!policy.allowExecutable) {
                    errors.add("executable binary not allowed: $path")
                } else {
                    warnings.add("executable binary: $path")
                }
                break
            }
        }

        for (magic in archiveMagicBytes) {
            if (content.size >= magic.size && content.copyOfRange(0, magic.size).contentEquals(magic)) {
                if (!policy.allowNestedArchive && ext != ".zip" && ext != ".amitiax") {
                    warnings.add("archive magic without expected extension: $path")
                }
                break
            }
        }

        checkSecretPatterns(path, content, warnings)

        return EntryValidationResult(
            passed = errors.isEmpty(),
            warnings = warnings,
            errors = errors
        )
    }

    private fun checkSecretPatterns(path: String, content: ByteArray, warnings: MutableList<String>) {
        if (content.isEmpty()) return

        val text = try {
            String(content, Charsets.UTF_8)
        } catch (e: Exception) {
            return
        }

        for (indicator in secretIndicators) {
            if (text.contains(indicator)) {
                warnings.add("potential secret detected in: $path ($indicator...)")
                break
            }
        }

        val pathLower = path.lowercase()
        if (pathLower == ".env" || pathLower.endsWith(".env")) {
            warnings.add("environment file detected: $path")
        }
    }

    private fun getFileExtension(path: String): String {
        val lastDot = path.lastIndexOf('.')
        val lastSlash = path.lastIndexOf('/')
        if (lastDot > lastSlash && lastDot >= 0) {
            return path.substring(lastDot)
        }
        return ""
    }
}
