package com.amitia.amitia_app.runtime.manifest

@Suppress("unused")
internal data class RuntimeManifestInput(
    val runtimeManifest: RuntimeManifest,
    val guestLayoutSha256: String,
    val mountContractSha256: String,
)

internal interface RuntimeManifestBuilder {
    fun build(): RuntimeManifestResult
}
