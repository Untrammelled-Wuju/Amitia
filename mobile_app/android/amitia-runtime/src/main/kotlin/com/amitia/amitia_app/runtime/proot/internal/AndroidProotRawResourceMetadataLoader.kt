package com.amitia.amitia_app.runtime.proot.internal

import android.content.Context
import com.amitia.amitia_app.runtime.proot.ProotArtifact
import com.amitia.amitia_app.runtime.proot.ProotError
import com.amitia.amitia_app.runtime.proot.ProotErrorCode

internal class AndroidProotRawResourceMetadataLoader(private val context: Context, private val resourceId: Int) : ProotMetadataLoaderInternal, ProotMetadataVerifier {
    override fun load(): ProotArtifact? = when (val r = loadArtifact()) { is ProotMetadataResult.Success -> r.artifact; is ProotMetadataResult.Error -> null }
    override fun loadArtifact(): ProotMetadataResult = try {
        val json = context.resources.openRawResource(resourceId).use { it.bufferedReader().use { r -> r.readText() } }
        val obj = org.json.JSONObject(json)
        val version = obj.optString("version", null) ?: return ProotMetadataResult.Error(ProotError.of(ProotErrorCode.METADATA_INVALID, "missing version"))
        val sha256 = obj.optString("sha256", null) ?: return ProotMetadataResult.Error(ProotError.of(ProotErrorCode.METADATA_INVALID, "missing sha256"))
        try { ProotMetadataResult.Success(ProotArtifact.create(version = version, sha256 = sha256)) }
        catch (e: IllegalArgumentException) { ProotMetadataResult.Error(ProotError.of(ProotErrorCode.METADATA_INVALID, e.message ?: "invalid metadata")) }
    } catch (e: Exception) { ProotMetadataResult.Error(ProotError.of(ProotErrorCode.METADATA_MISSING, "failed to load: ${e.message}")) }
}