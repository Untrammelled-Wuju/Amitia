package com.amitia.amitia_app.nativeprovider.screencapture

import android.accessibilityservice.AccessibilityService
import android.graphics.Bitmap
import android.graphics.ColorSpace
import android.os.Build
import android.util.Base64
import androidx.annotation.RequiresApi
import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.accessibility.AccessibilityServiceRegistry
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse
import java.io.ByteArrayOutputStream
import java.security.MessageDigest
import java.util.concurrent.atomic.AtomicLong
import kotlin.coroutines.resume
import kotlinx.coroutines.suspendCancellableCoroutine

/**
 * Screen capture provider used by the Go Android automation runtime.
 *
 * API 30+ AccessibilityService.takeScreenshot is used because it is durable
 * across application process restarts and is display-aware. The bridge returns
 * a bounded PNG payload plus a content hash; the Go side persists it under the
 * configured data root and exposes an amitia:// resource URI to OCR/VLM.
 */
internal class ScreenCaptureNativeHandler : AndroidNativeOperationHandler {

    private val captureGeneration = AtomicLong(0L)

    override val operations: Set<String> = setOf(OP_STATUS, OP_CAPTURE)

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse {
        return when (request.operation) {
            OP_STATUS -> status(request)
            OP_CAPTURE -> capture(request)
            else -> error(request, NativeBridgeProtocol.ERR_OPERATION_NOT_SUPPORTED, "unknown screen capture operation")
        }
    }

    private fun status(request: NativeBridgeRequest): NativeBridgeResponse {
        val supported = Build.VERSION.SDK_INT >= Build.VERSION_CODES.R
        val connected = AccessibilityServiceRegistry.isServiceConnected()
        return success(
            request,
            mapOf(
                "supported" to supported,
                "available" to (supported && connected),
                "state" to when {
                    !supported -> "unsupported"
                    !connected -> "permission_required"
                    else -> "ready"
                },
                "provider" to "accessibility_take_screenshot",
                "reason" to when {
                    !supported -> "AccessibilityService.takeScreenshot requires Android 11+"
                    !connected -> "accessibility service is not connected"
                    else -> null
                },
            ),
        )
    }

    private suspend fun capture(request: NativeBridgeRequest): NativeBridgeResponse {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.R) {
            return error(request, "SCREEN_CAPTURE_UNSUPPORTED", "screen capture requires Android 11+")
        }
        val service = AccessibilityServiceRegistry.current()
            ?: return error(request, "SCREEN_CAPTURE_PERMISSION_REQUIRED", "accessibility service is not connected")
        val displayId = (request.payload["displayId"] as? Number)?.toInt() ?: 0
        val maxWidth = ((request.payload["maxWidth"] as? Number)?.toInt() ?: DEFAULT_MAX_WIDTH).coerceIn(320, 4096)
        val maxHeight = ((request.payload["maxHeight"] as? Number)?.toInt() ?: DEFAULT_MAX_HEIGHT).coerceIn(320, 4096)
        val format = (request.payload["format"] as? String)?.trim()?.lowercase() ?: FORMAT_PNG
        if (format !in SUPPORTED_FORMATS) {
            return error(request, "SCREEN_CAPTURE_INVALID_FORMAT", "unsupported screenshot format: $format")
        }
        val defaultQuality = if (format == FORMAT_PNG) 100 else DEFAULT_LOSSY_QUALITY
        val quality = (request.payload["quality"] as? Number)?.toInt() ?: defaultQuality
        if (quality !in 1..100) {
            return error(request, "SCREEN_CAPTURE_INVALID_QUALITY", "screenshot quality must be between 1 and 100")
        }

        return captureApi30(service, request, displayId, maxWidth, maxHeight, format, quality)
    }

    @RequiresApi(Build.VERSION_CODES.R)
    private suspend fun captureApi30(
        service: AccessibilityService,
        request: NativeBridgeRequest,
        displayId: Int,
        maxWidth: Int,
        maxHeight: Int,
        format: String,
        quality: Int,
    ): NativeBridgeResponse = suspendCancellableCoroutine { continuation ->
        service.takeScreenshot(
            displayId,
            service.mainExecutor,
            object : AccessibilityService.TakeScreenshotCallback {
                override fun onSuccess(result: AccessibilityService.ScreenshotResult) {
                    val buffer = result.hardwareBuffer
                    try {
                        val hardwareBitmap = Bitmap.wrapHardwareBuffer(buffer, ColorSpace.get(ColorSpace.Named.SRGB))
                        if (hardwareBitmap == null) {
                            if (continuation.isActive) {
                                continuation.resume(error(request, "SCREEN_CAPTURE_FAILED", "unable to wrap screenshot hardware buffer"))
                            }
                            return
                        }

                        val screenWidth = hardwareBitmap.width
                        val screenHeight = hardwareBitmap.height
                        val software = hardwareBitmap.copy(Bitmap.Config.ARGB_8888, false)
                        val encodedBitmap = downscaleIfNeeded(software, maxWidth, maxHeight)
                        if (encodedBitmap !== software) software.recycle()

                        val compressFormat = when (format) {
                            FORMAT_JPEG -> Bitmap.CompressFormat.JPEG
                            FORMAT_WEBP -> Bitmap.CompressFormat.WEBP_LOSSY
                            else -> Bitmap.CompressFormat.PNG
                        }
                        val mimeType = when (format) {
                            FORMAT_JPEG -> "image/jpeg"
                            FORMAT_WEBP -> "image/webp"
                            else -> "image/png"
                        }
                        val compressionQuality = if (format == FORMAT_PNG) 100 else quality
                        val bytes = ByteArrayOutputStream().use { stream ->
                            if (!encodedBitmap.compress(compressFormat, compressionQuality, stream)) {
                                ByteArray(0)
                            } else {
                                stream.toByteArray()
                            }
                        }
                        val imageWidth = encodedBitmap.width
                        val imageHeight = encodedBitmap.height
                        encodedBitmap.recycle()

                        if (bytes.isEmpty()) {
                            if (continuation.isActive) {
                                continuation.resume(error(request, "SCREEN_CAPTURE_FAILED", "failed to encode screenshot"))
                            }
                            return
                        }
                        if (bytes.size > MAX_BRIDGE_BYTES) {
                            if (continuation.isActive) {
                                continuation.resume(error(request, "SCREEN_CAPTURE_TOO_LARGE", "encoded screenshot exceeds bridge limit"))
                            }
                            return
                        }

                        val generation = captureGeneration.incrementAndGet()
                        val stateToken = sha256(bytes)
                        if (continuation.isActive) {
                            continuation.resume(
                                success(
                                    request,
                                    mapOf(
                                        "provider" to "accessibility_take_screenshot",
                                        "displayId" to displayId,
                                        "screenWidth" to screenWidth,
                                        "screenHeight" to screenHeight,
                                        "imageWidth" to imageWidth,
                                        "imageHeight" to imageHeight,
                                        "capturedAt" to System.currentTimeMillis(),
                                        "generation" to generation,
                                        "stateToken" to stateToken,
                                        "format" to format,
                                        "mime" to mimeType,
                                        "dataBase64" to Base64.encodeToString(bytes, Base64.NO_WRAP),
                                    ),
                                ),
                            )
                        }
                    } finally {
                        buffer.close()
                    }
                }

                override fun onFailure(errorCode: Int) {
                    if (continuation.isActive) {
                        continuation.resume(
                            error(
                                request,
                                "SCREEN_CAPTURE_FAILED",
                                "AccessibilityService.takeScreenshot failed with code $errorCode",
                            ),
                        )
                    }
                }
            },
        )
    }

    private fun downscaleIfNeeded(bitmap: Bitmap, maxWidth: Int, maxHeight: Int): Bitmap {
        if (bitmap.width <= maxWidth && bitmap.height <= maxHeight) return bitmap
        val scale = minOf(maxWidth.toDouble() / bitmap.width, maxHeight.toDouble() / bitmap.height)
        val width = (bitmap.width * scale).toInt().coerceAtLeast(1)
        val height = (bitmap.height * scale).toInt().coerceAtLeast(1)
        return Bitmap.createScaledBitmap(bitmap, width, height, true)
    }

    private fun sha256(bytes: ByteArray): String =
        MessageDigest.getInstance("SHA-256").digest(bytes).joinToString("") { "%02x".format(it) }

    private fun success(request: NativeBridgeRequest, result: Map<String, Any?>): NativeBridgeResponse =
        NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = result,
        )

    private fun error(request: NativeBridgeRequest, code: String, message: String): NativeBridgeResponse =
        NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_ERROR,
            error = NativeBridgeError(code = code, message = message),
        )

    companion object {
        const val OP_STATUS = "screen_capture.status"
        const val OP_CAPTURE = "screen_capture.capture"
        private const val DEFAULT_MAX_WIDTH = 1440
        private const val DEFAULT_MAX_HEIGHT = 2560
        private const val DEFAULT_LOSSY_QUALITY = 90
        private const val FORMAT_PNG = "png"
        private const val FORMAT_JPEG = "jpeg"
        private const val FORMAT_WEBP = "webp"
        private val SUPPORTED_FORMATS = setOf(FORMAT_PNG, FORMAT_JPEG, FORMAT_WEBP)
        private const val MAX_BRIDGE_BYTES = 16 * 1024 * 1024
    }
}
