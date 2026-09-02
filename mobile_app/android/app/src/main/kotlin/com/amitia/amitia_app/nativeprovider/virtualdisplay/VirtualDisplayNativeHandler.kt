package com.amitia.amitia_app.nativeprovider.virtualdisplay

import android.content.Context
import android.graphics.PixelFormat
import android.hardware.display.DisplayManager
import android.hardware.display.VirtualDisplay
import android.media.ImageReader
import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicLong

/** Owns real Android VirtualDisplay instances and returns system display IDs. */
internal class VirtualDisplayNativeHandler(
    context: Context,
) : AndroidNativeOperationHandler {

    private data class ManagedDisplay(
        val name: String,
        var width: Int,
        var height: Int,
        var densityDpi: Int,
        val virtualDisplay: VirtualDisplay,
        var imageReader: ImageReader,
        var generation: Long,
    )

    private val displayManager = context.applicationContext.getSystemService(DisplayManager::class.java)
    private val generation = AtomicLong(0L)
    private val displays = ConcurrentHashMap<Int, ManagedDisplay>()

    override val operations: Set<String> = setOf(OP_STATUS, OP_CREATE, OP_GET, OP_LIST, OP_RESIZE, OP_RELEASE)

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse = when (request.operation) {
        OP_STATUS -> handleStatus(request)
        OP_CREATE -> handleCreate(request)
        OP_GET -> handleGet(request)
        OP_LIST -> handleList(request)
        OP_RESIZE -> handleResize(request)
        OP_RELEASE -> handleRelease(request)
        else -> unsupportedOperation(request)
    }

    private fun handleStatus(request: NativeBridgeRequest): NativeBridgeResponse = success(
        request,
        mapOf(
            "supported" to (displayManager != null),
            "available" to (displayManager != null),
            "canCreate" to (displayManager != null),
            "activeCount" to displays.size,
            "activeDisplayIds" to displays.keys.sorted(),
            "generation" to generation.get(),
            "provider" to "android_display_manager",
            "state" to if (displayManager != null) "ready" else "unavailable",
        ),
    )

    private fun handleCreate(request: NativeBridgeRequest): NativeBridgeResponse {
        val manager = displayManager ?: return error(request, "VIRTUAL_DISPLAY_UNSUPPORTED", "DisplayManager is unavailable")
        val name = (request.payload["name"] as? String)?.trim().takeUnless { it.isNullOrEmpty() } ?: "amitia_virtual"
        val width = (request.payload["width"] as? Number)?.toInt() ?: 1080
        val height = (request.payload["height"] as? Number)?.toInt() ?: 1920
        val densityDpi = (request.payload["densityDpi"] as? Number)?.toInt() ?: 320
        if (width !in 1..4096 || height !in 1..4096 || densityDpi !in 72..1000) {
            return error(request, "VIRTUAL_DISPLAY_INVALID_PARAMETERS", "invalid virtual display parameters: ${width}x${height}@${densityDpi}")
        }

        return try {
            val reader = ImageReader.newInstance(width, height, PixelFormat.RGBA_8888, 2)
            val flags = DisplayManager.VIRTUAL_DISPLAY_FLAG_PRESENTATION or
                DisplayManager.VIRTUAL_DISPLAY_FLAG_PUBLIC or
                DisplayManager.VIRTUAL_DISPLAY_FLAG_OWN_CONTENT_ONLY
            val vd = manager.createVirtualDisplay(name, width, height, densityDpi, reader.surface, flags)
                ?: run {
                    reader.close()
                    return error(request, "VIRTUAL_DISPLAY_CREATE_FAILED", "DisplayManager.createVirtualDisplay returned null")
                }
            val displayId = vd.display.displayId
            if (displayId < 0) {
                vd.release()
                reader.close()
                return error(request, "VIRTUAL_DISPLAY_CREATE_FAILED", "system did not assign a valid display ID")
            }
            val gen = generation.incrementAndGet()
            displays[displayId] = ManagedDisplay(name, width, height, densityDpi, vd, reader, gen)
            success(request, info(displayId, displays.getValue(displayId)))
        } catch (t: Throwable) {
            error(request, "VIRTUAL_DISPLAY_CREATE_FAILED", t.message ?: t::class.java.simpleName)
        }
    }

    private fun handleGet(request: NativeBridgeRequest): NativeBridgeResponse {
        val displayId = (request.payload["displayId"] as? Number)?.toInt()
        val managed = when {
            displayId != null -> displays[displayId]
            displays.size == 1 -> displays.values.firstOrNull()
            else -> null
        } ?: return error(request, "VIRTUAL_DISPLAY_NOT_FOUND", "virtual display not found; provide displayId when multiple displays exist")
        val actualId = managed.virtualDisplay.display.displayId
        return success(request, info(actualId, managed))
    }

    private fun handleList(request: NativeBridgeRequest): NativeBridgeResponse = success(
        request,
        mapOf("displays" to displays.entries.sortedBy { it.key }.map { info(it.key, it.value) }),
    )

    private fun handleResize(request: NativeBridgeRequest): NativeBridgeResponse {
        val displayId = (request.payload["displayId"] as? Number)?.toInt()
            ?: return error(request, "VIRTUAL_DISPLAY_ID_MISMATCH", "displayId is required")
        val managed = displays[displayId]
            ?: return error(request, "VIRTUAL_DISPLAY_NOT_FOUND", "virtual display $displayId not found")
        val width = (request.payload["width"] as? Number)?.toInt() ?: managed.width
        val height = (request.payload["height"] as? Number)?.toInt() ?: managed.height
        val densityDpi = (request.payload["densityDpi"] as? Number)?.toInt() ?: managed.densityDpi
        if (width !in 1..4096 || height !in 1..4096 || densityDpi !in 72..1000) {
            return error(request, "VIRTUAL_DISPLAY_INVALID_PARAMETERS", "invalid resize parameters")
        }

        return try {
            val newReader = ImageReader.newInstance(width, height, PixelFormat.RGBA_8888, 2)
            managed.virtualDisplay.setSurface(newReader.surface)
            managed.virtualDisplay.resize(width, height, densityDpi)
            managed.imageReader.close()
            managed.imageReader = newReader
            managed.width = width
            managed.height = height
            managed.densityDpi = densityDpi
            managed.generation = generation.incrementAndGet()
            success(request, info(displayId, managed))
        } catch (t: Throwable) {
            error(request, "VIRTUAL_DISPLAY_RESIZE_FAILED", t.message ?: t::class.java.simpleName)
        }
    }

    private fun handleRelease(request: NativeBridgeRequest): NativeBridgeResponse {
        val displayId = (request.payload["displayId"] as? Number)?.toInt()
            ?: return error(request, "VIRTUAL_DISPLAY_ID_MISMATCH", "displayId is required")
        val managed = displays.remove(displayId)
            ?: return error(request, "VIRTUAL_DISPLAY_NOT_FOUND", "virtual display $displayId not found")
        return try {
            managed.virtualDisplay.setSurface(null)
            managed.virtualDisplay.release()
            managed.imageReader.close()
            val gen = generation.incrementAndGet()
            success(request, mapOf("released" to true, "displayId" to displayId, "generation" to gen))
        } catch (t: Throwable) {
            error(request, "VIRTUAL_DISPLAY_NATIVE_ERROR", t.message ?: t::class.java.simpleName)
        }
    }

    private fun info(displayId: Int, managed: ManagedDisplay): Map<String, Any?> = mapOf(
        "displayId" to displayId,
        "name" to managed.name,
        "width" to managed.width,
        "height" to managed.height,
        "densityDpi" to managed.densityDpi,
        "generation" to managed.generation,
        "surfaceAttached" to managed.virtualDisplay.surface != null,
        "active" to displays.containsKey(displayId),
        "provider" to "android_display_manager",
    )

    private fun success(request: NativeBridgeRequest, result: Map<String, Any?>) = NativeBridgeResponse(
        protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
        requestId = request.requestId,
        status = NativeBridgeProtocol.STATUS_SUCCESS,
        result = result,
    )

    private fun error(request: NativeBridgeRequest, code: String, message: String) = NativeBridgeResponse(
        protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
        requestId = request.requestId,
        status = NativeBridgeProtocol.STATUS_ERROR,
        error = NativeBridgeError(code = code, message = message),
    )

    private fun unsupportedOperation(request: NativeBridgeRequest) = error(
        request,
        NativeBridgeProtocol.ERR_OPERATION_NOT_SUPPORTED,
        "unknown virtual display operation: ${request.operation}",
    )

    companion object {
        const val OP_STATUS = "virtual_display.status"
        const val OP_CREATE = "virtual_display.create"
        const val OP_GET = "virtual_display.get"
        const val OP_LIST = "virtual_display.list"
        const val OP_RESIZE = "virtual_display.resize"
        const val OP_RELEASE = "virtual_display.release"
    }
}
