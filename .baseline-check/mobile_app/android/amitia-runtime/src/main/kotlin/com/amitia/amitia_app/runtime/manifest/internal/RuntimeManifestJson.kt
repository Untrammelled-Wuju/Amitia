package com.amitia.amitia_app.runtime.manifest.internal

import com.amitia.amitia_app.runtime.manifest.RuntimeManifest
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestComponent
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestError
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestErrorCode
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestInstallation
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestPaths
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestPayload
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestTarget
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestVerification

internal object RuntimeManifestJson {

    fun serialize(manifest: RuntimeManifest): String {
        val root = mutableMapOf<String, Any?>()
        root[RuntimeManifest.JSON_SCHEMA_VERSION] = manifest.schemaVersion
        root[RuntimeManifest.JSON_RUNTIME_VERSION] = manifest.runtimeVersion
        root[RuntimeManifest.JSON_SOURCE_COMMIT] = manifest.sourceCommit
        root[RuntimeManifest.JSON_PACKAGE_ID] = manifest.packageId
        root[RuntimeManifest.JSON_PACKAGE_SHA256] = manifest.packageSha256
        root[RuntimeManifest.JSON_TARGET] = serializeTarget(manifest.target)
        root[RuntimeManifest.JSON_INSTALLATION] = serializeInstallation(manifest.installation)
        root[RuntimeManifest.JSON_PAYLOADS] = manifest.payloads.map { serializePayload(it) }
        root[RuntimeManifest.JSON_COMPONENTS] = manifest.components.map { serializeComponent(it) }
        root[RuntimeManifest.JSON_PATHS] = serializePaths(manifest.paths)
        root[RuntimeManifest.JSON_VERIFICATION] = serializeVerification(manifest.verification)
        return serializeSorted(root, 0)
    }

    private fun serializeTarget(t: RuntimeManifestTarget): Map<String, Any?> = linkedMapOf(
        RuntimeManifestTarget.JSON_HOST_PLATFORM to t.hostPlatform,
        RuntimeManifestTarget.JSON_HOST_ABI to t.hostAbi,
        RuntimeManifestTarget.JSON_RUNTIME_KIND to t.runtimeKind,
        RuntimeManifestTarget.JSON_GUEST_PLATFORM to t.guestPlatform,
        RuntimeManifestTarget.JSON_GUEST_ARCHITECTURE to t.guestArchitecture,
        RuntimeManifestTarget.JSON_DISTRIBUTION to t.distribution,
        RuntimeManifestTarget.JSON_DISTRIBUTION_RELEASE to t.distributionRelease,
    )

    private fun serializeInstallation(i: RuntimeManifestInstallation): Map<String, Any?> = linkedMapOf(
        RuntimeManifestInstallation.JSON_ACTIVE_VERSION to i.activeVersion,
        RuntimeManifestInstallation.JSON_ROOTFS_ID to i.rootfsId,
        RuntimeManifestInstallation.JSON_RUNTIME_ROOT_ID to i.runtimeRootId,
        RuntimeManifestInstallation.JSON_RUNTIME_ROOT_TREE_SHA256 to i.runtimeRootTreeSha256,
    )

    private fun serializePayload(p: RuntimeManifestPayload): Map<String, Any?> = linkedMapOf(
        RuntimeManifestPayload.JSON_ID to p.id,
        RuntimeManifestPayload.JSON_ROLE to p.role,
        RuntimeManifestPayload.JSON_SHA256 to p.sha256,
        RuntimeManifestPayload.JSON_SIZE to p.size,
    )

    private fun serializeComponent(c: RuntimeManifestComponent): Map<String, Any?> {
        val m = linkedMapOf<String, Any?>()
        m[RuntimeManifestComponent.JSON_ID] = c.id
        m[RuntimeManifestComponent.JSON_VERSION] = c.version ?: ""
        m[RuntimeManifestComponent.JSON_ARCHITECTURE] = c.architecture ?: ""
        m[RuntimeManifestComponent.JSON_ROOT] = c.root
        m[RuntimeManifestComponent.JSON_ENTRY] = c.entry ?: ""
        m[RuntimeManifestComponent.JSON_SHA256] = c.sha256 ?: ""
        m[RuntimeManifestComponent.JSON_TREE_SHA256] = c.treeSha256 ?: ""
        m[RuntimeManifestComponent.JSON_SOURCE] = c.source
        return m
    }

    private fun serializePaths(p: RuntimeManifestPaths): Map<String, Any?> = linkedMapOf(
        RuntimeManifestPaths.JSON_ROOTFS_HOST_PATH to p.rootfsHostPath,
        RuntimeManifestPaths.JSON_RUNTIME_ROOT_HOST_PATH to p.runtimeRootHostPath,
        RuntimeManifestPaths.JSON_CONFIG_HOST_PATH to p.configHostPath,
        RuntimeManifestPaths.JSON_DATA_HOST_PATH to p.dataHostPath,
        RuntimeManifestPaths.JSON_CACHE_HOST_PATH to p.cacheHostPath,
        RuntimeManifestPaths.JSON_LOG_HOST_PATH to p.logHostPath,
        RuntimeManifestPaths.JSON_RUN_HOST_PATH to p.runHostPath,
        RuntimeManifestPaths.JSON_GUEST_RUNTIME_ROOT to p.guestRuntimeRoot,
        RuntimeManifestPaths.JSON_GUEST_CONFIG_ROOT to p.guestConfigRoot,
        RuntimeManifestPaths.JSON_GUEST_DATA_ROOT to p.guestDataRoot,
        RuntimeManifestPaths.JSON_GUEST_CACHE_ROOT to p.guestCacheRoot,
        RuntimeManifestPaths.JSON_GUEST_LOG_ROOT to p.guestLogRoot,
        RuntimeManifestPaths.JSON_GUEST_RUN_ROOT to p.guestRunRoot,
    )

    private fun serializeVerification(v: RuntimeManifestVerification): Map<String, Any?> = linkedMapOf(
        RuntimeManifestVerification.JSON_PACKAGE_VERIFIED to v.packageVerified,
        RuntimeManifestVerification.JSON_ROOTFS_VERIFIED to v.rootfsVerified,
        RuntimeManifestVerification.JSON_RUNTIME_ROOT_VERIFIED to v.runtimeRootVerified,
        RuntimeManifestVerification.JSON_COMPONENTS_VERIFIED to v.componentsVerified,
        RuntimeManifestVerification.JSON_GUEST_LAYOUT_VERIFIED to v.guestLayoutVerified,
        RuntimeManifestVerification.JSON_MOUNT_CONTRACT_VERIFIED to v.mountContractVerified,
    )

    @Suppress("UNCHECKED_CAST")
    private fun serializeSorted(value: Any?, indent: Int): String {
        val pad = "  ".repeat(indent)
        val padInner = "  ".repeat(indent + 1)
        return when {
            value is Map<*, *> -> {
                val entries = (value as Map<String, Any?>).toList().sortedBy { it.first }
                if (entries.isEmpty()) "{}"
                else buildString {
                    append("{\n")
                    for ((idx, entry) in entries.withIndex()) {
                        append(padInner)
                        append("\"")
                        append(entry.first)
                        append("\": ")
                        append(serializeSorted(entry.second, indent + 1))
                        if (idx < entries.size - 1) append(",")
                        append("\n")
                    }
                    append(pad)
                    append("}")
                }
            }
            value is List<*> -> {
                if (value.isEmpty()) "[]"
                else buildString {
                    append("[\n")
                    for ((idx, item) in value.withIndex()) {
                        append(padInner)
                        append(serializeSorted(item, indent + 1))
                        if (idx < value.size - 1) append(",")
                        append("\n")
                    }
                    append(pad)
                    append("]")
                }
            }
            value is String -> {
                val escaped = value
                    .replace("\\", "\\\\")
                    .replace("\"", "\\\"")
                    .replace("\n", "\\n")
                    .replace("\r", "\\r")
                    .replace("\t", "\\t")
                "\"$escaped\""
            }
            value is Boolean -> value.toString()
            value is Number -> value.toString()
            value == null -> "null"
            else -> throw RuntimeManifestError(
                RuntimeManifestErrorCode.INTERNAL_ERROR,
                "cannot serialize type: ${value.javaClass.name}"
            )
        }
    }

    @Suppress("UNCHECKED_CAST")
    fun deserialize(json: String): RuntimeManifest {
        val root = parseValue(json.trim(), 0).value as Map<String, Any?>
        return RuntimeManifest(
            schemaVersion = (root[RuntimeManifest.JSON_SCHEMA_VERSION] as? Number)?.toInt()
                ?: throw RuntimeManifestError(
                    RuntimeManifestErrorCode.MANIFEST_INVALID_JSON,
                    "missing schemaVersion"
                ),
            runtimeVersion = root[RuntimeManifest.JSON_RUNTIME_VERSION] as? String
                ?: throw RuntimeManifestError(
                    RuntimeManifestErrorCode.MANIFEST_INVALID_JSON,
                    "missing runtimeVersion"
                ),
            sourceCommit = root[RuntimeManifest.JSON_SOURCE_COMMIT] as? String
                ?: throw RuntimeManifestError(
                    RuntimeManifestErrorCode.MANIFEST_INVALID_JSON,
                    "missing sourceCommit"
                ),
            packageId = root[RuntimeManifest.JSON_PACKAGE_ID] as? String
                ?: throw RuntimeManifestError(
                    RuntimeManifestErrorCode.MANIFEST_INVALID_JSON,
                    "missing packageId"
                ),
            packageSha256 = root[RuntimeManifest.JSON_PACKAGE_SHA256] as? String
                ?: throw RuntimeManifestError(
                    RuntimeManifestErrorCode.MANIFEST_INVALID_JSON,
                    "missing packageSha256"
                ),
            target = deserializeTarget(root[RuntimeManifest.JSON_TARGET] as? Map<String, Any?> ?: emptyMap()),
            installation = deserializeInstallation(root[RuntimeManifest.JSON_INSTALLATION] as? Map<String, Any?> ?: emptyMap()),
            payloads = (root[RuntimeManifest.JSON_PAYLOADS] as? List<Any?> ?: emptyList()).map {
                deserializePayload(it as? Map<String, Any?> ?: emptyMap())
            },
            components = (root[RuntimeManifest.JSON_COMPONENTS] as? List<Any?> ?: emptyList()).map {
                deserializeComponent(it as? Map<String, Any?> ?: emptyMap())
            },
            paths = deserializePaths(root[RuntimeManifest.JSON_PATHS] as? Map<String, Any?> ?: emptyMap()),
            verification = deserializeVerification(root[RuntimeManifest.JSON_VERIFICATION] as? Map<String, Any?> ?: emptyMap()),
        )
    }

    private fun deserializeTarget(m: Map<String, Any?>): RuntimeManifestTarget = RuntimeManifestTarget(
        hostPlatform = m[RuntimeManifestTarget.JSON_HOST_PLATFORM] as? String ?: "",
        hostAbi = m[RuntimeManifestTarget.JSON_HOST_ABI] as? String ?: "",
        runtimeKind = m[RuntimeManifestTarget.JSON_RUNTIME_KIND] as? String ?: "",
        guestPlatform = m[RuntimeManifestTarget.JSON_GUEST_PLATFORM] as? String ?: "",
        guestArchitecture = m[RuntimeManifestTarget.JSON_GUEST_ARCHITECTURE] as? String ?: "",
        distribution = m[RuntimeManifestTarget.JSON_DISTRIBUTION] as? String ?: "",
        distributionRelease = m[RuntimeManifestTarget.JSON_DISTRIBUTION_RELEASE] as? String ?: "",
    )

    private fun deserializeInstallation(m: Map<String, Any?>): RuntimeManifestInstallation =
        RuntimeManifestInstallation(
            activeVersion = m[RuntimeManifestInstallation.JSON_ACTIVE_VERSION] as? String ?: "",
            rootfsId = m[RuntimeManifestInstallation.JSON_ROOTFS_ID] as? String ?: "",
            runtimeRootId = m[RuntimeManifestInstallation.JSON_RUNTIME_ROOT_ID] as? String ?: "",
            runtimeRootTreeSha256 = m[RuntimeManifestInstallation.JSON_RUNTIME_ROOT_TREE_SHA256] as? String ?: "",
        )

    private fun deserializePayload(m: Map<String, Any?>): RuntimeManifestPayload = RuntimeManifestPayload(
        id = m[RuntimeManifestPayload.JSON_ID] as? String ?: "",
        role = m[RuntimeManifestPayload.JSON_ROLE] as? String ?: "",
        sha256 = m[RuntimeManifestPayload.JSON_SHA256] as? String ?: "",
        size = (m[RuntimeManifestPayload.JSON_SIZE] as? Number)?.toLong() ?: 0L,
    )

    private fun deserializeComponent(m: Map<String, Any?>): RuntimeManifestComponent = RuntimeManifestComponent(
        id = m[RuntimeManifestComponent.JSON_ID] as? String ?: "",
        version = (m[RuntimeManifestComponent.JSON_VERSION] as? String)?.takeIf { it.isNotEmpty() },
        architecture = (m[RuntimeManifestComponent.JSON_ARCHITECTURE] as? String)?.takeIf { it.isNotEmpty() },
        root = m[RuntimeManifestComponent.JSON_ROOT] as? String ?: "",
        entry = (m[RuntimeManifestComponent.JSON_ENTRY] as? String)?.takeIf { it.isNotEmpty() },
        sha256 = (m[RuntimeManifestComponent.JSON_SHA256] as? String)?.takeIf { it.isNotEmpty() },
        treeSha256 = (m[RuntimeManifestComponent.JSON_TREE_SHA256] as? String)?.takeIf { it.isNotEmpty() },
        source = m[RuntimeManifestComponent.JSON_SOURCE] as? String ?: "",
    )

    private fun deserializePaths(m: Map<String, Any?>): RuntimeManifestPaths = RuntimeManifestPaths(
        rootfsHostPath = m[RuntimeManifestPaths.JSON_ROOTFS_HOST_PATH] as? String ?: "",
        runtimeRootHostPath = m[RuntimeManifestPaths.JSON_RUNTIME_ROOT_HOST_PATH] as? String ?: "",
        configHostPath = m[RuntimeManifestPaths.JSON_CONFIG_HOST_PATH] as? String ?: "",
        dataHostPath = m[RuntimeManifestPaths.JSON_DATA_HOST_PATH] as? String ?: "",
        cacheHostPath = m[RuntimeManifestPaths.JSON_CACHE_HOST_PATH] as? String ?: "",
        logHostPath = m[RuntimeManifestPaths.JSON_LOG_HOST_PATH] as? String ?: "",
        runHostPath = m[RuntimeManifestPaths.JSON_RUN_HOST_PATH] as? String ?: "",
        guestRuntimeRoot = m[RuntimeManifestPaths.JSON_GUEST_RUNTIME_ROOT] as? String ?: "",
        guestConfigRoot = m[RuntimeManifestPaths.JSON_GUEST_CONFIG_ROOT] as? String ?: "",
        guestDataRoot = m[RuntimeManifestPaths.JSON_GUEST_DATA_ROOT] as? String ?: "",
        guestCacheRoot = m[RuntimeManifestPaths.JSON_GUEST_CACHE_ROOT] as? String ?: "",
        guestLogRoot = m[RuntimeManifestPaths.JSON_GUEST_LOG_ROOT] as? String ?: "",
        guestRunRoot = m[RuntimeManifestPaths.JSON_GUEST_RUN_ROOT] as? String ?: "",
    )

    private fun deserializeVerification(m: Map<String, Any?>): RuntimeManifestVerification =
        RuntimeManifestVerification(
            packageVerified = m[RuntimeManifestVerification.JSON_PACKAGE_VERIFIED] as? Boolean ?: false,
            rootfsVerified = m[RuntimeManifestVerification.JSON_ROOTFS_VERIFIED] as? Boolean ?: false,
            runtimeRootVerified = m[RuntimeManifestVerification.JSON_RUNTIME_ROOT_VERIFIED] as? Boolean ?: false,
            componentsVerified = m[RuntimeManifestVerification.JSON_COMPONENTS_VERIFIED] as? Boolean ?: false,
            guestLayoutVerified = m[RuntimeManifestVerification.JSON_GUEST_LAYOUT_VERIFIED] as? Boolean ?: false,
            mountContractVerified = m[RuntimeManifestVerification.JSON_MOUNT_CONTRACT_VERIFIED] as? Boolean ?: false,
        )

    data class ParseResult(val value: Any?, val nextPos: Int)

    private fun parseValue(input: String, start: Int): ParseResult {
        var pos = skipSpaces(input, start)
        if (pos >= input.length) throw parseError("unexpected end", pos)
        return when (val c = input[pos]) {
            '{' -> parseObject(input, pos)
            '[' -> parseArray(input, pos)
            '"' -> parseStringFull(input, pos)
            't' -> { if (input.startsWith("true", pos)) ParseResult(true, pos + 4) else throw parseError("expected true", pos) }
            'f' -> { if (input.startsWith("false", pos)) ParseResult(false, pos + 5) else throw parseError("expected false", pos) }
            'n' -> { if (input.startsWith("null", pos)) ParseResult(null, pos + 4) else throw parseError("expected null", pos) }
            '-' -> parseNumberFull(input, pos)
            in '0'..'9' -> parseNumberFull(input, pos)
            else -> throw parseError("unexpected char '$c'", pos)
        }
    }

    private fun parseObject(input: String, start: Int): ParseResult {
        var pos = skipSpaces(input, start + 1)
        val result = mutableMapOf<String, Any?>()
        if (pos < input.length && input[pos] == '}') return ParseResult(result, pos + 1)
        while (true) {
            pos = skipSpaces(input, pos)
            if (pos >= input.length || input[pos] != '"') throw parseError("expected \"", pos)
            val keyResult = parseStringFull(input, pos)
            val key = keyResult.value as String
            pos = skipSpaces(input, keyResult.nextPos)
            if (pos >= input.length || input[pos] != ':') throw parseError("expected :", pos)
            pos = skipSpaces(input, pos + 1)
            val valResult = parseValue(input, pos)
            result[key] = valResult.value
            pos = valResult.nextPos
            pos = skipSpaces(input, pos)
            if (pos >= input.length) throw parseError("unexpected end in object", pos)
            when (input[pos]) {
                '}' -> return ParseResult(result, pos + 1)
                ',' -> pos++
                else -> throw parseError("expected } or ,", pos)
            }
        }
    }

    private fun parseArray(input: String, start: Int): ParseResult {
        var pos = skipSpaces(input, start + 1)
        val result = mutableListOf<Any?>()
        if (pos < input.length && input[pos] == ']') return ParseResult(result, pos + 1)
        while (true) {
            pos = skipSpaces(input, pos)
            val valResult = parseValue(input, pos)
            result.add(valResult.value)
            pos = valResult.nextPos
            pos = skipSpaces(input, pos)
            if (pos >= input.length) throw parseError("unexpected end in array", pos)
            when (input[pos]) {
                ']' -> return ParseResult(result, pos + 1)
                ',' -> pos++
                else -> throw parseError("expected ] or ,", pos)
            }
        }
    }

    private fun parseStringFull(input: String, start: Int): ParseResult {
        if (input[start] != '"') throw parseError("expected \" at start", start)
        var pos = start + 1
        val sb = StringBuilder()
        while (pos < input.length) {
            val c = input[pos]
            if (c == '"') return ParseResult(sb.toString(), pos + 1)
            if (c == '\\') {
                if (pos + 1 >= input.length) throw parseError("unexpected end in string", pos)
                pos++
                when (input[pos]) {
                    '"' -> sb.append('"')
                    '\\' -> sb.append('\\')
                    '/' -> sb.append('/')
                    'n' -> sb.append('\n')
                    'r' -> sb.append('\r')
                    't' -> sb.append('\t')
                    else -> { sb.append('\\'); sb.append(input[pos]) }
                }
            } else {
                sb.append(c)
            }
            pos++
        }
        throw parseError("unterminated string", pos)
    }

    private fun parseNumberFull(input: String, start: Int): ParseResult {
        var pos = start
        if (input[pos] == '-') pos++
        while (pos < input.length && input[pos].isDigit()) pos++
        val numStr = input.substring(start, pos)
        val num: Number = if (numStr.contains('.')) numStr.toDouble() else numStr.toLong()
        return ParseResult(num, pos)
    }

    private fun skipSpaces(input: String, start: Int): Int {
        var pos = start
        while (pos < input.length && input[pos].isWhitespace()) pos++
        return pos
    }

    private fun parseError(message: String, pos: Int): RuntimeManifestError =
        RuntimeManifestError(RuntimeManifestErrorCode.MANIFEST_INVALID_JSON, "$message at pos $pos")
}
