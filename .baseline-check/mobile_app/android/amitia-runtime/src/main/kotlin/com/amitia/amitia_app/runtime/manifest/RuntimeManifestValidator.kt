package com.amitia.amitia_app.runtime.manifest

internal interface RuntimeManifestValidator {
    fun validateSchema(schemaVersion: Int): RuntimeManifestError?
    fun validateRuntimeVersion(runtimeVersion: String): RuntimeManifestError?
    fun validateSourceCommit(sourceCommit: String): RuntimeManifestError?
    fun validatePackageSha256(packageSha256: String): RuntimeManifestError?
    fun validateTarget(target: RuntimeManifestTarget): RuntimeManifestError?
    fun validateInstallation(installation: RuntimeManifestInstallation, runtimeVersion: String): RuntimeManifestError?
    fun validateComponents(components: List<RuntimeManifestComponent>): RuntimeManifestError?
    fun validatePayloads(payloads: List<RuntimeManifestPayload>): RuntimeManifestError?
    fun validatePaths(paths: RuntimeManifestPaths): RuntimeManifestError?
    fun validateVerification(verification: RuntimeManifestVerification): RuntimeManifestError?
}
