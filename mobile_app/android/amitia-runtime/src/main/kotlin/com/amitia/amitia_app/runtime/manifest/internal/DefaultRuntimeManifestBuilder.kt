package com.amitia.amitia_app.runtime.manifest.internal

import com.amitia.amitia_app.runtime.manifest.RuntimeManifest
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestBuilder
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestError
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestErrorCode
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestResult

internal class DefaultRuntimeManifestBuilder : RuntimeManifestBuilder {

    private var manifest: RuntimeManifest? = null

    fun setManifest(manifest: RuntimeManifest): DefaultRuntimeManifestBuilder {
        this.manifest = manifest
        return this
    }

    override fun build(): RuntimeManifestResult {
        val m = manifest ?: return RuntimeManifestResult.failure(
            RuntimeManifestError(
                RuntimeManifestErrorCode.MANIFEST_INVALID_JSON,
                "manifest is null"
            )
        )
        return RuntimeManifestResult.success(m)
    }
}
