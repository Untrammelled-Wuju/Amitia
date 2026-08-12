package com.amitia.amitia_app.nativeprovider.clipboard

import android.content.ClipData
import android.content.ClipDescription
import android.content.ClipboardManager
import android.content.Context
import android.os.Build
import java.util.concurrent.atomic.AtomicLong

internal class ClipboardNativeHandler(private val context: Context) {

    private val appContext = context.applicationContext
    private val stateReader = ClipboardStateReader(appContext)
    private val generation = AtomicLong(1L)

    fun currentGeneration(): Long = generation.get()

    fun onClipboardChanged() {
        generation.incrementAndGet()
    }

    fun execute(request: ClipboardNativeRequest): ClipboardNativeResponse {
        return when (request.operation) {
            OP_STATUS -> handleStatus(request)
            OP_READ -> handleRead(request)
            OP_WRITE -> handleWrite(request)
            OP_CLEAR -> handleClear(request)
            else -> ClipboardNativeResponse(
                requestId = request.requestId,
                status = "error",
                error = ClipboardNativeError(
                    code = "CLIPBOARD_UNSUPPORTED",
                    message = "unknown clipboard operation: ${request.operation}",
                ),
            )
        }
    }

    private fun handleStatus(request: ClipboardNativeRequest): ClipboardNativeResponse {
        val state = stateReader.readState()
        return ClipboardNativeResponse(
            requestId = request.requestId,
            status = "success",
            result = mapOf(
                "supported" to state.supported,
                "canWrite" to state.canWrite,
                "canRead" to state.canRead,
                "appForeground" to state.appForeground,
                "appHasInputFocus" to state.appHasInputFocus,
                "readRequiresForeground" to state.readRequiresForeground,
                "hasPrimaryClip" to state.hasPrimaryClip,
                "supportedMimeTypes" to state.supportedMimeTypes,
                "maxTextBytes" to state.maxTextBytes,
                "state" to state.state,
                "reason" to state.reason,
            ),
        )
    }

    private fun handleRead(request: ClipboardNativeRequest): ClipboardNativeResponse {
        val state = stateReader.readState()
        if (!state.canRead) {
            return ClipboardNativeResponse(
                requestId = request.requestId,
                status = "error",
                error = ClipboardNativeError(
                    code = "CLIPBOARD_READ_FOREGROUND_REQUIRED",
                    message = "clipboard read requires app foreground and input focus",
                ),
            )
        }

        if (!state.hasPrimaryClip) {
            return ClipboardNativeResponse(
                requestId = request.requestId,
                status = "success",
                result = mapOf(
                    "hasContent" to false,
                    "text" to null,
                    "mimeType" to "text/plain",
                    "itemCount" to 0,
                    "truncated" to false,
                    "sensitive" to false,
                    "generation" to generation.get(),
                ),
            )
        }

        val clipboardManager = try {
            appContext.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
        } catch (e: Exception) {
            return ClipboardNativeResponse(
                requestId = request.requestId,
                status = "error",
                error = ClipboardNativeError(
                    code = "CLIPBOARD_READ_FAILED",
                    message = "failed to access ClipboardManager: ${e.message}",
                ),
            )
        }

        val primaryClip = try {
            clipboardManager.primaryClip
        } catch (e: Exception) {
            return ClipboardNativeResponse(
                requestId = request.requestId,
                status = "error",
                error = ClipboardNativeError(
                    code = "CLIPBOARD_READ_FAILED",
                    message = "failed to read primary clip: ${e.message}",
                ),
            )
        }

        if (primaryClip == null || primaryClip.itemCount == 0) {
            return ClipboardNativeResponse(
                requestId = request.requestId,
                status = "success",
                result = mapOf(
                    "hasContent" to false,
                    "text" to null,
                    "mimeType" to "text/plain",
                    "itemCount" to 0,
                    "truncated" to false,
                    "sensitive" to false,
                    "generation" to generation.get(),
                ),
            )
        }

        val description = primaryClip.description
        val itemCount = primaryClip.itemCount
        val sensitive = isSensitive(description)

        val firstItem = primaryClip.getItemAt(0) ?: return ClipboardNativeResponse(
            requestId = request.requestId,
            status = "success",
            result = mapOf(
                "hasContent" to false,
                "text" to null,
                "mimeType" to "text/plain",
                "itemCount" to itemCount,
                "truncated" to false,
                "sensitive" to sensitive,
                "generation" to generation.get(),
            ),
        )

        val mimeType = if (description.hasMimeType(ClipDescription.MIMETYPE_TEXT_PLAIN)) {
            "text/plain"
        } else if (description.hasMimeType("text/html")) {
            "text/plain"
        } else if (description.hasMimeType(ClipDescription.MIMETYPE_TEXT_INTENT)) {
            return ClipboardNativeResponse(
                requestId = request.requestId,
                status = "error",
                error = ClipboardNativeError(
                    code = "CLIPBOARD_CONTENT_TYPE_UNSUPPORTED",
                    message = "intent clipboard content is not supported",
                ),
            )
        } else if (firstItem.uri != null) {
            return ClipboardNativeResponse(
                requestId = request.requestId,
                status = "error",
                error = ClipboardNativeError(
                    code = "CLIPBOARD_CONTENT_TYPE_UNSUPPORTED",
                    message = "URI clipboard content is not supported",
                ),
            )
        } else {
            return ClipboardNativeResponse(
                requestId = request.requestId,
                status = "error",
                error = ClipboardNativeError(
                    code = "CLIPBOARD_CONTENT_TYPE_UNSUPPORTED",
                    message = "clipboard content type is not supported",
                ),
            )
        }

        val text = firstItem.text?.toString() ?: return ClipboardNativeResponse(
            requestId = request.requestId,
            status = "success",
            result = mapOf(
                "hasContent" to false,
                "text" to null,
                "mimeType" to mimeType,
                "itemCount" to itemCount,
                "truncated" to false,
                "sensitive" to sensitive,
                "generation" to generation.get(),
            ),
        )

        if (text.length > MAX_TEXT_BYTES) {
            return ClipboardNativeResponse(
                requestId = request.requestId,
                status = "error",
                error = ClipboardNativeError(
                    code = "CLIPBOARD_CONTENT_TOO_LARGE",
                    message = "clipboard content exceeds maximum size",
                ),
            )
        }

        return ClipboardNativeResponse(
            requestId = request.requestId,
            status = "success",
            result = mapOf(
                "hasContent" to true,
                "text" to text,
                "mimeType" to mimeType,
                "itemCount" to itemCount,
                "truncated" to false,
                "sensitive" to sensitive,
                "generation" to generation.get(),
            ),
        )
    }

    private fun handleWrite(request: ClipboardNativeRequest): ClipboardNativeResponse {
        val text = request.payload["text"] as? String ?: return ClipboardNativeResponse(
            requestId = request.requestId,
            status = "error",
            error = ClipboardNativeError(
                code = "CLIPBOARD_INPUT_TOO_LARGE",
                message = "text is required",
            ),
        )

        val sensitive = request.payload["sensitive"] as? Boolean ?: false

        if (text.length > MAX_TEXT_BYTES) {
            return ClipboardNativeResponse(
                requestId = request.requestId,
                status = "error",
                error = ClipboardNativeError(
                    code = "CLIPBOARD_INPUT_TOO_LARGE",
                    message = "clipboard input exceeds maximum size",
                ),
            )
        }

        val clipboardManager = try {
            appContext.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
        } catch (e: Exception) {
            return ClipboardNativeResponse(
                requestId = request.requestId,
                status = "error",
                error = ClipboardNativeError(
                    code = "CLIPBOARD_WRITE_FAILED",
                    message = "failed to access ClipboardManager: ${e.message}",
                ),
            )
        }

        try {
            val clip = ClipData.newPlainText(CLIP_LABEL, text)
            if (sensitive && Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                val extras = android.os.PersistableBundle()
                extras.putBoolean(ClipDescription.EXTRA_IS_SENSITIVE, true)
                clip.description.extras = extras
            }

            clipboardManager.setPrimaryClip(clip)
            generation.incrementAndGet()

            return ClipboardNativeResponse(
                requestId = request.requestId,
                status = "success",
                result = mapOf(
                    "written" to true,
                    "bytes" to text.toByteArray().size,
                    "sensitive" to sensitive,
                    "generation" to generation.get(),
                ),
            )
        } catch (e: Exception) {
            return ClipboardNativeResponse(
                requestId = request.requestId,
                status = "error",
                error = ClipboardNativeError(
                    code = "CLIPBOARD_WRITE_FAILED",
                    message = "setPrimaryClip failed: ${e.message}",
                ),
            )
        }
    }

    private fun handleClear(request: ClipboardNativeRequest): ClipboardNativeResponse {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.P) {
            return ClipboardNativeResponse(
                requestId = request.requestId,
                status = "error",
                error = ClipboardNativeError(
                    code = "CLIPBOARD_UNSUPPORTED",
                    message = "clearPrimaryClip requires API 28+",
                ),
            )
        }

        val clipboardManager = try {
            appContext.getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager
        } catch (e: Exception) {
            return ClipboardNativeResponse(
                requestId = request.requestId,
                status = "error",
                error = ClipboardNativeError(
                    code = "CLIPBOARD_CLEAR_FAILED",
                    message = "failed to access ClipboardManager: ${e.message}",
                ),
            )
        }

        try {
            clipboardManager.clearPrimaryClip()
            generation.incrementAndGet()
            return ClipboardNativeResponse(
                requestId = request.requestId,
                status = "success",
                result = mapOf("cleared" to true),
            )
        } catch (e: Exception) {
            return ClipboardNativeResponse(
                requestId = request.requestId,
                status = "error",
                error = ClipboardNativeError(
                    code = "CLIPBOARD_CLEAR_FAILED",
                    message = "clearPrimaryClip failed: ${e.message}",
                ),
            )
        }
    }

    private fun isSensitive(description: ClipDescription): Boolean {
        return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            description.extras?.getBoolean(ClipDescription.EXTRA_IS_SENSITIVE) ?: false
        } else {
            false
        }
    }

    companion object {
        const val OP_STATUS = "clipboard.status"
        const val OP_READ = "clipboard.read_text"
        const val OP_WRITE = "clipboard.write_text"
        const val OP_CLEAR = "clipboard.clear"
        const val MAX_TEXT_BYTES = 65536
        const val CLIP_LABEL = "Amitia"
    }
}
