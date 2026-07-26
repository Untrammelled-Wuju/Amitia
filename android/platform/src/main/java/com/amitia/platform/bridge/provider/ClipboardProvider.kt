package com.amitia.platform.bridge.provider

import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import com.amitia.platform.bridge.CapabilityProvider
import com.amitia.platform.bridge.NativeActionRequest
import com.amitia.platform.bridge.NativeActionResult
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class ClipboardProvider @Inject constructor(
    @ApplicationContext private val context: Context
) : CapabilityProvider {

    override fun action(): String = "clipboard"

    override fun requiredPermission(): String? = null

    override suspend fun execute(request: NativeActionRequest): NativeActionResult {
        val op = request.params["op"] ?: "read"
        val clipboard = context.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
        return when (op) {
            "read" -> {
                val text = clipboard.primaryClip?.getItemAt(0)?.text?.toString().orEmpty()
                NativeActionResult.Success(mapOf("text" to text))
            }
            "write" -> {
                val text = request.params["text"] ?: return NativeActionResult.Failed("text required")
                clipboard.setPrimaryClip(ClipData.newPlainText("amitia", text))
                NativeActionResult.Success(mapOf("status" to "written"))
            }
            else -> NativeActionResult.Failed("unsupported op: $op")
        }
    }
}
