package com.amitia.amitia_app.runtime.manifest.internal

import com.amitia.amitia_app.runtime.manifest.RuntimeManifestReader
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestResult
import com.amitia.amitia_app.runtime.manifest.RuntimeManifestStore

internal class DefaultRuntimeManifestReader(
    private val store: RuntimeManifestStore,
) : RuntimeManifestReader {

    override fun read(): RuntimeManifestResult = store.read()
}
