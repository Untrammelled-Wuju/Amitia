package com.amitia.amitia_app.runtime.manifest

internal interface RuntimeManifestStore {
    fun read(): RuntimeManifestResult
    fun write(manifest: RuntimeManifest): RuntimeManifestResult
    fun delete(): RuntimeManifestResult
}
