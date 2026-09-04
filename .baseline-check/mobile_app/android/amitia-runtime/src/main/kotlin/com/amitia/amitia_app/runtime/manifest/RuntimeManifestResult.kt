package com.amitia.amitia_app.runtime.manifest

sealed class RuntimeManifestResult {
    data class Success(val manifest: RuntimeManifest) : RuntimeManifestResult()
    data class Failure(val error: RuntimeManifestError) : RuntimeManifestResult()

    companion object {
        fun success(manifest: RuntimeManifest): RuntimeManifestResult = Success(manifest)
        fun failure(error: RuntimeManifestError): RuntimeManifestResult = Failure(error)
    }
}
