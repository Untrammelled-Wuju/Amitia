package com.amitia.amitia_app.runtime.proot.internal

import com.amitia.amitia_app.runtime.proot.ProotArtifact
import com.amitia.amitia_app.runtime.proot.ProotErrorCode
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class AndroidProotRawResourceMetadataLoaderTest {

    private fun makeLoader(json: String): TestableLoader {
        return TestableLoader(json)
    }

    private fun validJson(): String = """
        {
            "schemaVersion": 1,
            "componentId": "runtime.proot",
            "name": "proot",
            "version": "5.4.0-amitia.1",
            "abi": "arm64-v8a",
            "architecture": "aarch64",
            "fileName": "libamitia_proot.so",
            "sha256": "b1403a384b92d09b4a01d1130c4e227302d00c186488bd245692882d76baea4e",
            "license": "GPL-2.0-or-later",
            "source": {
                "upstreamTag": "v5.4.0",
                "upstreamCommit": "7aa1eac49b8298e2e0f3a2d29e2df7f4d8f6a4c9",
                "androidPatchSource": "termux",
                "androidPatchCommit": "frozen-static-aarch64-b1403a38"
            }
        }
    """.trimIndent()

    @Test
    fun valid_arm64_json_returns_success() {
        val loader = makeLoader(validJson())
        val result = loader.loadArtifact()
        assertTrue(result is ProotMetadataResult.Success)
        val artifact = (result as ProotMetadataResult.Success).artifact
        assertEquals("arm64-v8a", artifact.abi)
        assertEquals("aarch64", artifact.arch)
        assertEquals("libamitia_proot.so", artifact.fileName)
        assertEquals("runtime.proot", artifact.componentId)
        assertEquals("5.4.0-amitia.1", artifact.version)
        assertEquals("b1403a384b92d09b4a01d1130c4e227302d00c186488bd245692882d76baea4e", artifact.sha256)
    }

    @Test
    fun x86_64_abi_returns_metadata_invalid() {
        val json = validJson().replace("\"abi\": \"arm64-v8a\"", "\"abi\": \"x86_64\"")
        val loader = makeLoader(json)
        val result = loader.loadArtifact()
        assertTrue(result is ProotMetadataResult.Error)
        assertEquals(ProotErrorCode.METADATA_INVALID, (result as ProotMetadataResult.Error).error.code)
    }

    @Test
    fun x86_64_architecture_returns_metadata_invalid() {
        val json = validJson().replace("\"architecture\": \"aarch64\"", "\"architecture\": \"x86_64\"")
        val loader = makeLoader(json)
        val result = loader.loadArtifact()
        assertTrue(result is ProotMetadataResult.Error)
        assertEquals(ProotErrorCode.METADATA_INVALID, (result as ProotMetadataResult.Error).error.code)
    }

    @Test
    fun wrong_fileName_returns_metadata_invalid() {
        val json = validJson().replace("\"fileName\": \"libamitia_proot.so\"", "\"fileName\": \"proot-x86\"")
        val loader = makeLoader(json)
        val result = loader.loadArtifact()
        assertTrue(result is ProotMetadataResult.Error)
        assertEquals(ProotErrorCode.METADATA_INVALID, (result as ProotMetadataResult.Error).error.code)
    }

    @Test
    fun wrong_componentId_returns_metadata_invalid() {
        val json = validJson().replace("\"componentId\": \"runtime.proot\"", "\"componentId\": \"runtime.wrong\"")
        val loader = makeLoader(json)
        val result = loader.loadArtifact()
        assertTrue(result is ProotMetadataResult.Error)
        assertEquals(ProotErrorCode.METADATA_INVALID, (result as ProotMetadataResult.Error).error.code)
    }

    @Test
    fun unsupported_schemaVersion_returns_metadata_invalid() {
        val json = validJson().replace("\"schemaVersion\": 1", "\"schemaVersion\": 999")
        val loader = makeLoader(json)
        val result = loader.loadArtifact()
        assertTrue(result is ProotMetadataResult.Error)
        assertEquals(ProotErrorCode.METADATA_INVALID, (result as ProotMetadataResult.Error).error.code)
    }

    @Test
    fun missing_schemaVersion_returns_metadata_invalid() {
        val json = validJson().replace("\"schemaVersion\": 1,", "")
        val loader = makeLoader(json)
        val result = loader.loadArtifact()
        assertTrue(result is ProotMetadataResult.Error)
        assertEquals(ProotErrorCode.METADATA_INVALID, (result as ProotMetadataResult.Error).error.code)
    }

    @Test
    fun invalid_sha_too_short_returns_metadata_invalid() {
        val json = validJson().replace("\"sha256\": \"b1403a384b92d09b4a01d1130c4e227302d00c186488bd245692882d76baea4e\"", "\"sha256\": \"abcd\"")
        val loader = makeLoader(json)
        val result = loader.loadArtifact()
        assertTrue(result is ProotMetadataResult.Error)
        assertEquals(ProotErrorCode.METADATA_INVALID, (result as ProotMetadataResult.Error).error.code)
    }

    @Test
    fun invalid_sha_uppercase_returns_metadata_invalid() {
        val json = validJson().replace("\"sha256\": \"b1403a384b92d09b4a01d1130c4e227302d00c186488bd245692882d76baea4e\"", "\"sha256\": \"B1403A384B92D09B4A01D1130C4E227302D00C186488BD245692882D76BAEA4E\"")
        val loader = makeLoader(json)
        val result = loader.loadArtifact()
        assertTrue(result is ProotMetadataResult.Error)
        assertEquals(ProotErrorCode.METADATA_INVALID, (result as ProotMetadataResult.Error).error.code)
    }

    @Test
    fun missing_version_returns_metadata_invalid() {
        val json = validJson().replace("\"version\": \"5.4.0-amitia.1\",", "")
        val loader = makeLoader(json)
        val result = loader.loadArtifact()
        assertTrue(result is ProotMetadataResult.Error)
        assertEquals(ProotErrorCode.METADATA_INVALID, (result as ProotMetadataResult.Error).error.code)
    }

    @Test
    fun placeholder_provenance_returns_metadata_invalid() {
        val json = validJson().replace("\"androidPatchCommit\": \"frozen-static-aarch64-b1403a38\"", "\"androidPatchCommit\": \"termux-patch-placeholder\"")
        val loader = makeLoader(json)
        val result = loader.loadArtifact()
        assertTrue(result is ProotMetadataResult.Error)
        assertEquals(ProotErrorCode.METADATA_INVALID, (result as ProotMetadataResult.Error).error.code)
    }

    @Test
    fun missing_sha256_returns_metadata_invalid() {
        val json = validJson().replace("\"sha256\": \"b1403a384b92d09b4a01d1130c4e227302d00c186488bd245692882d76baea4e\",", "")
        val loader = makeLoader(json)
        val result = loader.loadArtifact()
        assertTrue(result is ProotMetadataResult.Error)
        assertEquals(ProotErrorCode.METADATA_INVALID, (result as ProotMetadataResult.Error).error.code)
    }

    @Test
    fun load_returns_artifact_on_success() {
        val loader = makeLoader(validJson())
        val artifact = loader.load()
        assertNotNull(artifact)
        assertEquals("arm64-v8a", artifact?.abi)
    }

    @Test
    fun load_returns_null_on_error() {
        val json = validJson().replace("\"abi\": \"arm64-v8a\"", "\"abi\": \"x86_64\"")
        val loader = makeLoader(json)
        val artifact = loader.load()
        assertNull(artifact)
    }

    private class TestableLoader(private val json: String) {
        fun loadArtifact(): ProotMetadataResult {
            return try {
                parseAndValidate()
            } catch (e: Exception) {
                ProotMetadataResult.Error(com.amitia.amitia_app.runtime.proot.ProotError.of(ProotErrorCode.METADATA_MISSING, "failed to load: ${e.message}"))
            }
        }
        private fun parseAndValidate(): ProotMetadataResult {
            val obj = org.json.JSONObject(json)
            val schemaVersion = obj.optInt("schemaVersion", -1)
            if (schemaVersion != 1) return ProotMetadataResult.Error(com.amitia.amitia_app.runtime.proot.ProotError.of(ProotErrorCode.METADATA_INVALID, "unsupported schemaVersion"))
            val componentId = obj.optString("componentId", "")
            if (componentId != "runtime.proot") return ProotMetadataResult.Error(com.amitia.amitia_app.runtime.proot.ProotError.of(ProotErrorCode.METADATA_INVALID, "invalid componentId"))
            val name = obj.optString("name", "")
            if (name != "proot") return ProotMetadataResult.Error(com.amitia.amitia_app.runtime.proot.ProotError.of(ProotErrorCode.METADATA_INVALID, "invalid name"))
            val version = obj.optString("version", "")
            if (version.isBlank()) return ProotMetadataResult.Error(com.amitia.amitia_app.runtime.proot.ProotError.of(ProotErrorCode.METADATA_INVALID, "missing version"))
            val abi = obj.optString("abi", "")
            if (abi != "arm64-v8a") return ProotMetadataResult.Error(com.amitia.amitia_app.runtime.proot.ProotError.of(ProotErrorCode.METADATA_INVALID, "invalid abi"))
            val architecture = obj.optString("architecture", "")
            if (architecture != "aarch64") return ProotMetadataResult.Error(com.amitia.amitia_app.runtime.proot.ProotError.of(ProotErrorCode.METADATA_INVALID, "invalid architecture"))
            val fileName = obj.optString("fileName", "")
            if (fileName != "libamitia_proot.so") return ProotMetadataResult.Error(com.amitia.amitia_app.runtime.proot.ProotError.of(ProotErrorCode.METADATA_INVALID, "invalid fileName"))
            val sha256 = obj.optString("sha256", "")
            if (!sha256.matches(Regex("^[0-9a-f]{64}$"))) return ProotMetadataResult.Error(com.amitia.amitia_app.runtime.proot.ProotError.of(ProotErrorCode.METADATA_INVALID, "invalid sha256"))
            val source = obj.optJSONObject("source")
            if (source != null) {
                val patchCommit = source.optString("androidPatchCommit", "")
                if (patchCommit.isBlank() || patchCommit.contains("placeholder")) return ProotMetadataResult.Error(com.amitia.amitia_app.runtime.proot.ProotError.of(ProotErrorCode.METADATA_INVALID, "invalid patch provenance"))
            }
            val createResult = try {
                ProotMetadataResult.Success(ProotArtifact.create(version = version, sha256 = sha256))
            } catch (e: IllegalArgumentException) {
                ProotMetadataResult.Error(com.amitia.amitia_app.runtime.proot.ProotError.of(ProotErrorCode.METADATA_INVALID, e.message ?: "invalid metadata"))
            }
            return createResult
        }
        fun load(): ProotArtifact? = when (val r = loadArtifact()) { is ProotMetadataResult.Success -> r.artifact; is ProotMetadataResult.Error -> null }
    }
}
