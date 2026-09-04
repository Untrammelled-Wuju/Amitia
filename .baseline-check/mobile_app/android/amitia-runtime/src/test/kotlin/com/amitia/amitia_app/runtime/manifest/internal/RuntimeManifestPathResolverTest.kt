package com.amitia.amitia_app.runtime.manifest.internal

import org.junit.Assert.assertEquals
import org.junit.Test

class RuntimeManifestPathResolverTest {

    @Test
    fun manifestPath_from_metadataRoot() {
        val resolver = RuntimeManifestPathResolver("/data/meta")
        assertEquals("/data/meta/runtime-manifest.json", resolver.manifestPath())
    }

    @Test
    fun manifestShaPath_from_metadataRoot() {
        val resolver = RuntimeManifestPathResolver("/data/meta")
        assertEquals("/data/meta/runtime-manifest.json.sha256", resolver.manifestShaPath())
    }

    @Test
    fun manifestTempPath_usesTmpSuffix() {
        val resolver = RuntimeManifestPathResolver("/data/meta")
        assertEquals("/data/meta/runtime-manifest.json.tmp", resolver.manifestTempPath())
    }

    @Test
    fun resolve_returnsBothPaths() {
        val resolver = RuntimeManifestPathResolver("/data/meta")
        val resolved = resolver.resolve()
        assertEquals("/data/meta/runtime-manifest.json", resolved.manifestHostPath)
        assertEquals("/data/meta/runtime-manifest.json.sha256", resolved.manifestShaHostPath)
    }

    @Test
    fun blankMetadataRoot_throws() {
        try {
            RuntimeManifestPathResolver("")
            throw AssertionError("should throw")
        } catch (_: IllegalArgumentException) {
        }
    }
}
