package com.amitia.amitia_app.runtime.manifest

interface RuntimeManifestStore {
    fun read(): RuntimeManifestResult
    fun write(manifest: RuntimeManifest): RuntimeManifestResult
    fun delete(): RuntimeManifestResult
}
