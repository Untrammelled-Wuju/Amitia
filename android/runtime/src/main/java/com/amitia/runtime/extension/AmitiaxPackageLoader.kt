package com.amitia.runtime.extension

import com.amitia.runtime.extension.security.ArchivePolicy
import com.amitia.runtime.extension.security.ArchiveSecurityInspector
import com.amitia.runtime.extension.security.IntegrityFilesDoc
import com.amitia.runtime.extension.security.IntegrityMissingException
import com.amitia.runtime.extension.security.IntegrityTreeDoc
import com.amitia.runtime.extension.security.IntegrityVerifier
import com.amitia.runtime.extension.security.PackageFileConstants
import com.amitia.runtime.extension.security.PackageSignatureVerifier
import com.amitia.runtime.extension.security.PublisherTrustStore
import com.amitia.runtime.extension.security.RevocationList
import com.amitia.runtime.extension.security.SignatureDoc
import java.io.File
import java.io.InputStream
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
    val packageHash: String,
    val treeHash: String = "",
    val manifestHash: String = "",
    val signatureDoc: SignatureDoc? = null,
    val signatureVerified: Boolean = false,
    val securityWarnings: List<String> = emptyList()
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
    },
    private val policy: ArchivePolicy = ArchivePolicy.default(),
    private val trustStore: PublisherTrustStore? = null,
    private val revocationList: RevocationList? = null,
    private val requireSignature: Boolean = false
) {
    private val inspector = ArchiveSecurityInspector(policy)
    private val integrityVerifier = IntegrityVerifier()
    private val signatureVerifier = PackageSignatureVerifier(trustStore, revocationList)

    fun loadFromFile(file: File): Result<AmitiaxPackage> = runCatching {
        file.inputStream().use { stream -> loadFromStream(stream) }
    }

    fun loadFromStream(stream: InputStream): AmitiaxPackage {
        val (entries, summary) = inspector.inspectAndExtract(stream)

        val manifestBytes = entries[PackageFileConstants.MANIFEST_FILE]
            ?: throw AmitiaxLoadException("manifest.json not found in package")

        val manifestRaw = manifestBytes.toString(Charsets.UTF_8)
        val manifest = json.decodeFromString(ExtensionManifest.serializer(), manifestRaw)

        validateManifest(manifest)

        val integrityFilesBytes = entries[PackageFileConstants.INTEGRITY_FILES]
            ?: throw IntegrityMissingException("integrity/files.json missing in package")

        val integrityTreeBytes = entries[PackageFileConstants.INTEGRITY_TREE]
            ?: throw IntegrityMissingException("integrity/content-tree.json missing in package")

        val integrityFilesDoc = json.decodeFromString(
            IntegrityFilesDoc.serializer(),
            integrityFilesBytes.toString(Charsets.UTF_8)
        )

        val integrityTreeDoc = json.decodeFromString(
            IntegrityTreeDoc.serializer(),
            integrityTreeBytes.toString(Charsets.UTF_8)
        )

        val skipPaths = setOf(
            PackageFileConstants.MANIFEST_FILE,
            PackageFileConstants.INTEGRITY_FILES,
            PackageFileConstants.INTEGRITY_TREE,
            PackageFileConstants.SIGNATURE_FILE,
            PackageFileConstants.V2_SIGNATURE_FILE
        )

        integrityVerifier.verifyIntegrity(
            packageFiles = entries,
            integrityFiles = integrityFilesDoc,
            integrityTree = integrityTreeDoc,
            skipPaths = skipPaths
        )

        val treeHash = integrityTreeDoc.treeHash

        val manifestHash = integrityVerifier.computeManifestHash(manifestRaw)

        integrityVerifier.verifyManifestContentTreeHash(
            manifestContentTreeHash = manifest.integrity?.contentTreeHash,
            treeHash = treeHash
        )

        val signatureDocBytes = entries[PackageFileConstants.SIGNATURE_FILE]
        val signatureDoc = if (signatureDocBytes != null) {
            json.decodeFromString(SignatureDoc.serializer(), signatureDocBytes.toString(Charsets.UTF_8))
        } else null

        val signatureVerified = if (signatureDoc != null) {
            val result = signatureVerifier.verify(
                signature = signatureDoc,
                treeHash = treeHash,
                manifestHash = manifestHash
            )
            if (!result.verified && requireSignature) {
                throw AmitiaxLoadException(
                    "signature verification failed: ${result.error}"
                )
            }
            result.verified
        } else {
            if (requireSignature) {
                throw AmitiaxLoadException("signature required but not found in package")
            }
            false
        }

        val modules = entries.filterKeys { it.startsWith("modules/") }
            .mapKeys { it.key.removePrefix("modules/") }
        val resources = entries.filterKeys { it.startsWith("resources/") }
            .mapKeys { it.key.removePrefix("resources/") }
        val assets = entries.filterKeys { it.startsWith("assets/") }
            .mapKeys { it.key.removePrefix("assets/") }
        val signatures = entries.filterKeys { it.startsWith("signatures/") || it.startsWith("META-INF/") }
            .filterKeys { it.endsWith(".json") || it.endsWith(".sig") }

        val packageHash = integrityVerifier.computePackageHash(entries)

        val warnings = summary.warnings.toMutableList()
        if (signatureDoc == null) {
            warnings.add("package has no signature")
        } else if (!signatureVerified && trustStore != null) {
            warnings.add("signature present but verification failed")
        } else if (trustStore == null && signatureDoc != null) {
            warnings.add("signature present but no trust store available for verification")
        }

        return AmitiaxPackage(
            manifest = manifest,
            manifestRaw = manifestRaw,
            modules = modules,
            resources = resources,
            assets = assets,
            signatures = signatures,
            integrityFiles = integrityFilesBytes.toString(Charsets.UTF_8),
            integrityTree = integrityTreeBytes.toString(Charsets.UTF_8),
            packageHash = packageHash,
            treeHash = treeHash,
            manifestHash = manifestHash,
            signatureDoc = signatureDoc,
            signatureVerified = signatureVerified,
            securityWarnings = warnings
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
}

class AmitiaxLoadException(message: String) : RuntimeException(message)
