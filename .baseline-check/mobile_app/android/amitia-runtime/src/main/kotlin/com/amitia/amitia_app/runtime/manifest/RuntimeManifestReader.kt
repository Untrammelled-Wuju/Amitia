package com.amitia.amitia_app.runtime.manifest

internal interface RuntimeManifestReader {
    fun read(): RuntimeManifestResult
}
