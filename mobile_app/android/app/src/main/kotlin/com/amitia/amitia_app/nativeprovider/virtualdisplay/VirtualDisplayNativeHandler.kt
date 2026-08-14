package com.amitia.amitia_app.nativeprovider.virtualdisplay

import android.content.Context
import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse
import java.util.concurrent.atomic.AtomicLong

internal class VirtualDisplayNativeHandler(
    private val context: Context,
) : AndroidNativeOperationHandler {

    private val generation = AtomicLong(0L)
    private val activeDisplay = AtomicLong(-1L)

    override val operations: Set<String> = setOf(OP_STATUS, OP_CREATE, OP_GET, OP_RESIZE, OP_RELEASE)

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse {
        return when (request.operation) {
            OP_STATUS -> handleStatus(request)
            OP_CREATE -> handleCreate(request)
            OP_GET -> handleGet(request)
            OP_RESIZE -> handleResize(request)
            OP_RELEASE -> handleRelease(request)
            else -> unsupportedOperation(request)
        }
    }

    private fun handleStatus(request: NativeBridgeRequest): NativeBridgeResponse {
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "activeDisplayId" to if (activeDisplay.get() >= 0) activeDisplay.get() else null,
                "generation" to generation.get(),
            ),
        )
    }

    private fun handleCreate(request: NativeBridgeRequest): NativeBridgeResponse {
        val name = request.payload["name"] as? String ?: "amitia_virtual"
        val width = (request.payload["width"] as? Number)?.toInt() ?: 1080
        val height = (request.payload["height"] as? Number)?.toInt() ?: 1920
        val densityDpi = (request.payload["densityDpi"] as? Number)?.toInt() ?: 320

        if (width <= 0 || height <= 0 || densityDpi <= 0) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "VIRTUAL_DISPLAY_INVALID_PARAMETERS",
                    message = "invalid virtual display parameters: ${width}x${height}@${densityDpi}",
                ),
            )
        }

        generation.incrementAndGet()
        activeDisplay.set(generation.get())

        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "displayId" to activeDisplay.get(),
                "name" to name,
                "width" to width,
                "height" to height,
                "densityDpi" to densityDpi,
                "generation" to generation.get(),
            ),
        )
    }

    private fun handleGet(request: NativeBridgeRequest): NativeBridgeResponse {
        val currentId = activeDisplay.get()
        if (currentId < 0) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "VIRTUAL_DISPLAY_NOT_FOUND",
                    message = "no active virtual display",
                ),
            )
        }
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "displayId" to currentId,
                "generation" to generation.get(),
            ),
        )
    }

    private fun handleResize(request: NativeBridgeRequest): NativeBridgeResponse {
        val displayId = (request.payload["displayId"] as? Number)?.toInt()
        val width = (request.payload["width"] as? Number)?.toInt() ?: 1080
        val height = (request.payload["height"] as? Number)?.toInt() ?: 1920
        val densityDpi = (request.payload["densityDpi"] as? Number)?.toInt() ?: 320

        if (activeDisplay.get() < 0) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "VIRTUAL_DISPLAY_NOT_FOUND",
                    message = "no active virtual display to resize",
                ),
            )
        }

        generation.incrementAndGet()
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "displayId" to activeDisplay.get(),
                "width" to width,
                "height" to height,
                "densityDpi" to densityDpi,
                "generation" to generation.get(),
            ),
        )
    }

    private fun handleRelease(request: NativeBridgeRequest): NativeBridgeResponse {
        val displayId = (request.payload["displayId"] as? Number)?.toInt()

        if (activeDisplay.get() < 0) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "VIRTUAL_DISPLAY_NOT_FOUND",
                    message = "no active virtual display to release",
                ),
            )
        }

        generation.incrementAndGet()
        activeDisplay.set(-1L)

        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "released" to true,
                "generation" to generation.get(),
            ),
        )
    }

    private fun unsupportedOperation(request: NativeBridgeRequest): NativeBridgeResponse {
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_ERROR,
            error = NativeBridgeError(
                code = NativeBridgeProtocol.ERR_OPERATION_NOT_SUPPORTED,
                message = "unknown virtual display operation: ${request.operation}",
            ),
        )
    }

    companion object {
        const val OP_STATUS = "virtual_display.status"
        const val OP_CREATE = "virtual_display.create"
        const val OP_GET = "virtual_display.get"
        const val OP_RESIZE = "virtual_display.resize"
        const val OP_RELEASE = "virtual_display.release"
    }
}
