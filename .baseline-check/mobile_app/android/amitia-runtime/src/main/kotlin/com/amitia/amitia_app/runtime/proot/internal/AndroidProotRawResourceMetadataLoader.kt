package com.amitia.amitia_app.runtime.proot.internal

import android.content.Context
import com.amitia.amitia_app.runtime.proot.ProotArtifact
import com.amitia.amitia_app.runtime.proot.ProotError
import com.amitia.amitia_app.runtime.proot.ProotErrorCode

internal class AndroidProotRawResourceMetadataLoader(private val context: Context, private val resourceId: Int) : ProotMetadataLoaderInternal, ProotMetadataVerifier {
    override fun load(): ProotArtifact? {
        return when (val r = loadArtifact()) {
            is ProotMetadataResult.Success -> r.artifact
            is ProotMetadataResult.Error -> null
        }
    }
    override fun loadArtifact(): ProotMetadataResult {
        return try {
            parseAndValidate()
        } catch (e: Exception) {
            ProotMetadataResult.Error(ProotError.of(ProotErrorCode.METADATA_MISSING, "failed to load: ${e.message}"))
        }
    }
    private fun parseAndValidate(): ProotMetadataResult {
        val json = context.resources.openRawResource(resourceId).use { it.bufferedReader().use { r -> r.readText() } }
        val obj = org.json.JSONObject(json)
        val schemaVersion = obj.optInt("schemaVersion", -1)
        if (schemaVersion != 1) return ProotMetadataResult.Error(ProotError.of(ProotErrorCode.METADATA_INVALID, "unsupported schemaVersion"))
        val componentId = obj.optString("componentId", "")
        if (componentId != "runtime.proot") return ProotMetadataResult.Error(ProotError.of(ProotErrorCode.METADATA_INVALID, "invalid componentId"))
        val name = obj.optString("name", "")
        if (name != "proot") return ProotMetadataResult.Error(ProotError.of(ProotErrorCode.METADATA_INVALID, "invalid name"))
        val version = obj.optString("version", "")
        if (version.isBlank()) return ProotMetadataResult.Error(ProotError.of(ProotErrorCode.METADATA_INVALID, "missing version"))
        val abi = obj.optString("abi", "")
        if (abi != "arm64-v8a") return ProotMetadataResult.Error(ProotError.of(ProotErrorCode.METADATA_INVALID, "invalid abi"))
        val architecture = obj.optString("architecture", "")
        if (architecture != "aarch64") return ProotMetadataResult.Error(ProotError.of(ProotErrorCode.METADATA_INVALID, "invalid architecture"))
        val fileName = obj.optString("fileName", "")
        if (fileName != "libamitia_proot.so") return ProotMetadataResult.Error(ProotError.of(ProotErrorCode.METADATA_INVALID, "invalid fileName"))
        val sha256 = obj.optString("sha256", "")
        if (!sha256.matches(Regex("^[0-9a-f]{64}$"))) return ProotMetadataResult.Error(ProotError.of(ProotErrorCode.METADATA_INVALID, "invalid sha256"))
        val source = obj.optJSONObject("source")
        if (source != null) {
            val patchCommit = source.optString("androidPatchCommit", "")
            if (patchCommit.isBlank() || patchCommit.contains("placeholder")) return ProotMetadataResult.Error(ProotError.of(ProotErrorCode.METADATA_INVALID, "invalid patch provenance"))
        }
        val createResult = try {
            ProotMetadataResult.Success(ProotArtifact.create(version = version, sha256 = sha256))
        } catch (e: IllegalArgumentException) {
            ProotMetadataResult.Error(ProotError.of(ProotErrorCode.METADATA_INVALID, e.message ?: "invalid metadata"))
        }
        return createResult
    }
}
