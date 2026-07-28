package com.amitia.runtime.extension

import java.io.File
import java.io.InputStream
import java.security.MessageDigest
import java.util.zip.ZipInputStream
import kotlinx.serialization.json.Json

data class AmitiaxPackage(
    val manifest: ExtensionManifest,
    val manifestRaw: String,
    val modules: Map<String, ByteArray>,
    val resources: Map<String, ByteArray>,
    val assets: Map<String, ByteArray>,
    val signatures: Map<String, ByteArray>,
    val integrityFiles: String?,
    val integrityTree: String?,
    val packageHash: String
) {
    fun moduleEntry(moduleId: String): ByteArray? = modules[moduleId]

    fun resource(path: String): ByteArray? = resources[path]

    fun asset(path: String): ByteArray? = assets[path]
}

class AmitiaxPackageLoader(
    private val json: Json = Json {
        ignoreUnknownKeys = true
        isLenient = true
        coerceInputValues = true
        explicitNulls = false
    }
) {
    fun loadFromFile(file: File): Result<AmitiaxPackage> = runCatching {
        file.inputStream().use { stream -> loadFromStream(stream) }
    }

    fun loadFromStream(stream: InputStream): AmitiaxPackage {
        val entries = mutableMapOf<String, ByteArray>()
        val manifestPath = "manifest.json"

        ZipInputStream(stream).use { zis ->
            var entry = zis.nextEntry
            while (entry != null) {
                if (!entry.isDirectory) {
                    val data = zis.readBytes()
                    entries[entry.name] = data
                }
                entry = zis.nextEntry
            }
        }

        val manifestBytes = entries[manifestPath]
            ?: entries.entries.firstOrNull { it.key.endsWith("/manifest.json") }?.value
            ?: throw AmitiaxLoadException("manifest.json not found in package")

        val manifestRaw = manifestBytes.toString(Charsets.UTF_8)
        val manifest = json.decodeFromString(ExtensionManifest.serializer(), manifestRaw)

        validateManifest(manifest)

        val modules = entries.filterKeys { it.startsWith("modules/") }
            .mapKeys { it.key.removePrefix("modules/") }
        val resources = entries.filterKeys { it.startsWith("resources/") }
            .mapKeys { it.key.removePrefix("resources/") }
        val assets = entries.filterKeys { it.startsWith("assets/") }
            .mapKeys { it.key.removePrefix("assets/") }
        val signatures = entries.filterKeys { it.startsWith("signatures/") || it.startsWith("META-INF/") }
            .filterKeys { it.endsWith(".json") || it.endsWith(".sig") }

        val integrityFiles = entries["integrity/files.json"]?.toString(Charsets.UTF_8)
        val integrityTree = entries["integrity/content-tree.json"]?.toString(Charsets.UTF_8)

        val packageHash = computePackageHash(entries)

        return AmitiaxPackage(
            manifest = manifest,
            manifestRaw = manifestRaw,
            modules = modules,
            resources = resources,
            assets = assets,
            signatures = signatures,
            integrityFiles = integrityFiles,
            integrityTree = integrityTree,
            packageHash = packageHash
        )
    }

    private fun validateManifest(manifest: ExtensionManifest) {
        if (manifest.extension.id.isBlank()) {
            throw AmitiaxLoadException("manifest extension.id must not be blank")
        }
        if (manifest.extension.version.isBlank()) {
            throw AmitiaxLoadException("manifest extension.version must not be blank")
        }
        if (manifest.extension.name.default.isBlank()) {
            throw AmitiaxLoadException("manifest extension.name.default must not be blank")
        }
        if (manifest.manifestVersion < 2) {
            throw AmitiaxLoadException("manifest version must be >= 2, got ${manifest.manifestVersion}")
        }
        if (manifest.modules.isEmpty()) {
            throw AmitiaxLoadException("manifest must define at least one module")
        }
        manifest.modules.forEach { module ->
            if (module.id.isBlank()) {
                throw AmitiaxLoadException("module id must not be blank")
            }
            if (module.type.isBlank()) {
                throw AmitiaxLoadException("module ${module.id} type must not be blank")
            }
        }
    }

    private fun computePackageHash(entries: Map<String, ByteArray>): String {
        val digest = MessageDigest.getInstance("SHA-256")
        entries.toSortedMap().forEach { (name, data) ->
            digest.update(name.toByteArray(Charsets.UTF_8))
            digest.update(0)
            digest.update(data)
            digest.update(0)
        }
        return digest.digest().joinToString("") { "%02x".format(it) }
    }
}

class AmitiaxLoadException(message: String) : RuntimeException(message)
