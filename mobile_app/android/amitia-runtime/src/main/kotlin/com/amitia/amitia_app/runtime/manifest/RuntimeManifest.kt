package com.amitia.amitia_app.runtime.manifest

data class RuntimeManifest(
    val schemaVersion: Int,
    val runtimeVersion: String,
    val sourceCommit: String,
    val packageId: String,
    val packageSha256: String,
    val target: RuntimeManifestTarget,
    val installation: RuntimeManifestInstallation,
    val payloads: List<RuntimeManifestPayload>,
    val components: List<RuntimeManifestComponent>,
    val paths: RuntimeManifestPaths,
    val verification: RuntimeManifestVerification,
) {
    init {
        require(schemaVersion == 1) { "unsupported schemaVersion: $schemaVersion" }
        require(runtimeVersion.isNotBlank()) { "runtimeVersion must not be blank" }
        require(sourceCommit.trim().length in 7..40) {
            "sourceCommit must be 7-40 hex chars, got: ${sourceCommit.trim().length}"
        }
        require(packageId.isNotBlank()) { "packageId must not be blank" }
        require(packageSha256.trim().length == 64) { "packageSha256 must be 64 hex chars" }
        require(payloads.isNotEmpty()) { "payloads must not be empty" }
        require(components.isNotEmpty()) { "components must not be empty" }
    }

    companion object {
        const val SCHEMA_VERSION: Int = 1
        const val JSON_SCHEMA_VERSION: String = "schemaVersion"
        const val JSON_RUNTIME_VERSION: String = "runtimeVersion"
        const val JSON_SOURCE_COMMIT: String = "sourceCommit"
        const val JSON_PACKAGE_ID: String = "packageId"
        const val JSON_PACKAGE_SHA256: String = "packageSha256"
        const val JSON_TARGET: String = "target"
        const val JSON_INSTALLATION: String = "installation"
        const val JSON_PAYLOADS: String = "payloads"
        const val JSON_COMPONENTS: String = "components"
        const val JSON_PATHS: String = "paths"
        const val JSON_VERIFICATION: String = "verification"
    }
}
