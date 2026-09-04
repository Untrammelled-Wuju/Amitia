package com.amitia.amitia_app.runtime.manifest

internal enum class RuntimeManifestVerifyMode {
    LIGHT,
    FULL,
}

internal interface RuntimeManifestVerifier {
    fun verify(manifest: RuntimeManifest, mode: RuntimeManifestVerifyMode): RuntimeManifestResult
}
