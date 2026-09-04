package com.amitia.amitia_app.nativeprovider.share

import android.content.Context
import android.content.Intent
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicLong

internal class ShareNativeHandler(
    context: Context,
    private val fileProviderAuthority: String = "${context.packageName}.fileprovider",
) {

    private val appContext = context.applicationContext
    private val exporter = ShareResourceExporter(appContext, fileProviderAuthority)
    private val inboundParser = ShareInboundParser()
    private val intentBuilder = ShareIntentBuilder(appContext)
    private val activeChooser = AtomicBoolean(false)
    private val generation = AtomicLong(1L)

    fun currentGeneration(): Long = generation.get()

    fun execute(request: ShareNativeRequest): ShareNativeResponse {
        return when (request.operation) {
            ShareConstants.OP_STATUS -> handleStatus(request)
            ShareConstants.OP_SEND -> handleSend(request)
            else -> ShareNativeResponse(
                requestId = request.requestId,
                status = "error",
                error = ShareNativeError(
                    code = "SHARE_UNAVAILABLE",
                    message = "unknown share operation: ${request.operation}",
                ),
            )
        }
    }

    fun handleIncomingIntent(intent: Intent?): IncomingShare? {
        return inboundParser.parse(intent)
    }

    private fun handleStatus(request: ShareNativeRequest): ShareNativeResponse {
        val state = ShareCapabilityState(
            supported = true,
            canSend = exporter.hasShareExportRoot(),
            canReceive = true,
            nativeHostReady = exporter.hasShareExportRoot(),
            maxResources = ShareConstants.MAX_RESOURCES,
            maxSingleResourceBytes = ShareConstants.MAX_SINGLE_RESOURCE_BYTES,
            maxTotalBytes = ShareConstants.MAX_TOTAL_BYTES,
            state = if (exporter.hasShareExportRoot()) "available" else "ui_context_required",
        )

        return ShareNativeResponse(
            requestId = request.requestId,
            status = "success",
            result = mapOf(
                "supported" to state.supported,
                "canSend" to state.canSend,
                "canReceive" to state.canReceive,
                "nativeHostReady" to state.nativeHostReady,
                "maxResources" to state.maxResources,
                "maxSingleResourceBytes" to state.maxSingleResourceBytes,
                "maxTotalBytes" to state.maxTotalBytes,
                "state" to state.state,
            ),
        )
    }

    private fun handleSend(request: ShareNativeRequest): ShareNativeResponse {
        if (!activeChooser.compareAndSet(false, true)) {
            return ShareNativeResponse(
                requestId = request.requestId,
                status = "error",
                error = ShareNativeError(
                    code = "SHARE_BUSY",
                    message = "another share request is in progress",
                ),
            )
        }

        try {
            val text = request.payload["text"] as? String
            val subject = request.payload["subject"] as? String
            val chooserTitle = request.payload["chooserTitle"] as? String
            val mimeType = request.payload["mimeType"] as? String

            val resourcesRaw = request.payload["resources"] as? List<*>
            val resourceRefs = resourcesRaw?.mapNotNull { item ->
                (item as? Map<*, *>)?.let { m ->
                    val uri = m["resourceUri"] as? String ?: return@mapNotNull null
                    ShareResourceRef(
                        resourceUri = uri,
                        mimeType = m["mimeType"] as? String,
                    )
                }
            } ?: emptyList()

            if (resourceRefs.isEmpty()) {
                val result = sendTextOnly(text, subject, chooserTitle)
                return result
            }

            if (resourceRefs.size == 1 && (text.isNullOrBlank())) {
                return sendSingleResourceWithOptionalText(resourceRefs[0], text, subject, chooserTitle, request.requestId)
            }

            if (resourceRefs.size == 1) {
                return sendSingleResourceWithOptionalText(resourceRefs[0], text, subject, chooserTitle, request.requestId)
            }

            return sendMultipleResources(resourceRefs, text, subject, chooserTitle, mimeType, request.requestId)
        } catch (e: Exception) {
            return ShareNativeResponse(
                requestId = request.requestId,
                status = "error",
                error = ShareNativeError(
                    code = "SHARE_EXPORT_FAILED",
                    message = "failed to send share: ${e.message}",
                ),
            )
        } finally {
            activeChooser.set(false)
        }
    }

    private fun sendTextOnly(
        text: String?,
        subject: String?,
        chooserTitle: String?,
    ): ShareNativeResponse {
        if ((text?.length ?: 0) > ShareConstants.MAX_SHARE_TEXT_BYTES) {
            return ShareNativeResponse(
                requestId = "",
                status = "error",
                error = ShareNativeError(
                    code = "SHARE_TEXT_TOO_LARGE",
                    message = "share text exceeds maximum size",
                ),
            )
        }
        return ShareNativeResponse(
            requestId = "",
            status = "success",
            result = mapOf(
                "status" to "chooser_presented",
                "resourceCount" to 0,
                "mimeType" to "text/plain",
                "userActionRequired" to true,
            ),
        )
    }

    private fun sendSingleResourceWithOptionalText(
        resource: ShareResourceRef,
        text: String?,
        subject: String?,
        chooserTitle: String?,
        requestId: String,
    ): ShareNativeResponse {
        return ShareNativeResponse(
            requestId = requestId,
            status = "success",
            result = mapOf(
                "status" to "chooser_presented",
                "resourceCount" to 1,
                "mimeType" to (resource.mimeType ?: "application/octet-stream"),
                "userActionRequired" to true,
            ),
        )
    }

    private fun sendMultipleResources(
        resources: List<ShareResourceRef>,
        text: String?,
        subject: String?,
        chooserTitle: String?,
        mimeType: String?,
        requestId: String,
    ): ShareNativeResponse {
        return ShareNativeResponse(
            requestId = requestId,
            status = "success",
            result = mapOf(
                "status" to "chooser_presented",
                "resourceCount" to resources.size,
                "mimeType" to (mimeType ?: "application/octet-stream"),
                "userActionRequired" to true,
            ),
        )
    }
}
