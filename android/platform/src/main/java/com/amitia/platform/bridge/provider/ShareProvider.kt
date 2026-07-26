package com.amitia.platform.bridge.provider

import android.content.Context
import android.content.Intent
import com.amitia.platform.bridge.CapabilityProvider
import com.amitia.platform.bridge.NativeActionRequest
import com.amitia.platform.bridge.NativeActionResult
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class ShareProvider @Inject constructor(
    @ApplicationContext private val context: Context
) : CapabilityProvider {

    override fun action(): String = "share"

    override fun requiredPermission(): String? = null

    override suspend fun execute(request: NativeActionRequest): NativeActionResult {
        val text = request.params["text"] ?: return NativeActionResult.Failed("text required")
        return try {
            val intent = Intent(Intent.ACTION_SEND).apply {
                type = "text/plain"
                putExtra(Intent.EXTRA_TEXT, text)
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            }
            context.startActivity(Intent.createChooser(intent, "分享").apply {
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            })
            NativeActionResult.Success(mapOf("status" to "shared"))
        } catch (t: Throwable) {
            NativeActionResult.Failed(t.message ?: "share_failed", t)
        }
    }
}
