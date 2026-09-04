package com.amitia.amitia_app.runtime.manifest.internal

internal data class ResolvedManifestPaths(
    val manifestHostPath: String,
    val manifestShaHostPath: String,
)

internal class RuntimeManifestPathResolver(
    private val metadataRoot: String,
) {
    init {
        require(metadataRoot.isNotBlank()) { "metadataRoot must not be blank" }
    }

    fun manifestPath(): String = "$metadataRoot/runtime-manifest.json"

    fun manifestShaPath(): String = "$metadataRoot/runtime-manifest.json.sha256"

    fun manifestTempPath(): String = "$metadataRoot/runtime-manifest.json.tmp"

    fun manifestShaTempPath(): String = "$metadataRoot/runtime-manifest.json.sha256.tmp"

    fun resolve(): ResolvedManifestPaths = ResolvedManifestPaths(
        manifestHostPath = manifestPath(),
        manifestShaHostPath = manifestShaPath(),
    )

    companion object {
        const val MANIFEST_FILE: String = "runtime-manifest.json"
        const val MANIFEST_SHA_FILE: String = "runtime-manifest.json.sha256"
        const val TEMP_SUFFIX: String = ".tmp"
    }
}
