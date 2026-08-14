package com.amitia.amitia_app.nativeprovider.overlay

import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Build
import android.provider.Settings
import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.atomic.AtomicLong

internal class OverlayNativeHandler(
    private val context: Context,
) : AndroidNativeOperationHandler {

    private val generation = AtomicLong(0L)
    private val activeOverlays = ConcurrentHashMap<String, OverlayInfo>()

    override fun supports(operation: String): Boolean {
        return operation == OP_STATUS ||
            operation == OP_REQUEST_PERMISSION ||
            operation == OP_CREATE ||
            operation == OP_UPDATE ||
            operation == OP_SHOW ||
            operation == OP_HIDE ||
            operation == OP_CLOSE ||
            operation == OP_LIST ||
            operation == OP_CLOSE_ALL
    }

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse {
        return when (request.operation) {
            OP_STATUS -> handleStatus(request)
            OP_REQUEST_PERMISSION -> handleRequestPermission(request)
            OP_CREATE -> handleCreate(request)
            OP_UPDATE -> handleUpdate(request)
            OP_SHOW -> handleShow(request)
            OP_HIDE -> handleHide(request)
            OP_CLOSE -> handleClose(request)
            OP_LIST -> handleList(request)
            OP_CLOSE_ALL -> handleCloseAll(request)
            else -> unsupportedOperation(request)
        }
    }

    private fun handleStatus(request: NativeBridgeRequest): NativeBridgeResponse {
        val canDraw = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            Settings.canDrawOverlays(context)
        } else {
            true
        }
        val state = OverlayCapabilityState(
            supported = true,
            permissionGranted = canDraw,
            permissionRequired = Build.VERSION.SDK_INT >= Build.VERSION_CODES.M,
            canDrawOverlays = canDraw,
            activeOverlays = activeOverlays.size,
            state = if (canDraw) "available" else "permission_required",
            reason = if (canDraw) "" else "overlay permission not granted",
        )
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "supported" to state.supported,
                "permissionGranted" to state.permissionGranted,
                "permissionRequired" to state.permissionRequired,
                "canDrawOverlays" to state.canDrawOverlays,
                "activeOverlays" to state.activeOverlays,
                "state" to state.state,
                "reason" to state.reason,
            ),
        )
    }

    private fun handleRequestPermission(request: NativeBridgeRequest): NativeBridgeResponse {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) {
            if (!Settings.canDrawOverlays(context)) {
                return NativeBridgeResponse(
                    protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                    requestId = request.requestId,
                    status = NativeBridgeProtocol.STATUS_SUCCESS,
                    result = mapOf(
                        "requested" to true,
                        "userActionRequired" to true,
                        "state" to "permission_pending",
                    ),
                )
            }
        }
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "requested" to false,
                "granted" to true,
                "userActionRequired" to false,
                "state" to "available",
            ),
        )
    }

    private fun handleCreate(request: NativeBridgeRequest): NativeBridgeResponse {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M && !Settings.canDrawOverlays(context)) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "OVERLAY_PERMISSION_REQUIRED",
                    message = "overlay permission not granted",
                    domainCode = "OVERLAY_PERMISSION_REQUIRED",
                ),
            )
        }

        val overlayId = request.payload["overlayId"] as? String
            ?: "overlay_${System.currentTimeMillis()}_${activeOverlays.size}"

        if (activeOverlays.containsKey(overlayId)) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "OVERLAY_ALREADY_EXISTS",
                    message = "overlay already exists: $overlayId",
                ),
            )
        }

        generation.incrementAndGet()
        val info = OverlayInfo(
            overlayId = overlayId,
            visible = false,
            width = (request.payload["width"] as? Number)?.toInt() ?: -1,
            height = (request.payload["height"] as? Number)?.toInt() ?: -1,
            x = (request.payload["x"] as? Number)?.toInt() ?: 0,
            y = (request.payload["y"] as? Number)?.toInt() ?: 0,
            generation = generation.get(),
        )
        activeOverlays[overlayId] = info

        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "overlayId" to overlayId,
                "created" to true,
                "generation" to generation.get(),
            ),
        )
    }

    private fun handleUpdate(request: NativeBridgeRequest): NativeBridgeResponse {
        val overlayId = request.payload["overlayId"] as? String
            ?: return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "OVERLAY_INVALID_REQUEST",
                    message = "overlayId is required",
                ),
            )

        val existing = activeOverlays[overlayId]
            ?: return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "OVERLAY_NOT_FOUND",
                    message = "overlay not found: $overlayId",
                ),
            )

        generation.incrementAndGet()
        val updated = existing.copy(generation = generation.get())
        activeOverlays[overlayId] = updated

        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "overlayId" to overlayId,
                "updated" to true,
                "generation" to generation.get(),
            ),
        )
    }

    private fun handleShow(request: NativeBridgeRequest): NativeBridgeResponse {
        val overlayId = request.payload["overlayId"] as? String
            ?: return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "OVERLAY_INVALID_REQUEST",
                    message = "overlayId is required",
                ),
            )

        val existing = activeOverlays[overlayId]
            ?: return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "OVERLAY_NOT_FOUND",
                    message = "overlay not found: $overlayId",
                ),
            )

        generation.incrementAndGet()
        activeOverlays[overlayId] = existing.copy(visible = true, generation = generation.get())

        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "overlayId" to overlayId,
                "shown" to true,
                "generation" to generation.get(),
            ),
        )
    }

    private fun handleHide(request: NativeBridgeRequest): NativeBridgeResponse {
        val overlayId = request.payload["overlayId"] as? String
            ?: return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "OVERLAY_INVALID_REQUEST",
                    message = "overlayId is required",
                ),
            )

        val existing = activeOverlays[overlayId]
            ?: return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "OVERLAY_NOT_FOUND",
                    message = "overlay not found: $overlayId",
                ),
            )

        generation.incrementAndGet()
        activeOverlays[overlayId] = existing.copy(visible = false, generation = generation.get())

        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "overlayId" to overlayId,
                "hidden" to true,
                "generation" to generation.get(),
            ),
        )
    }

    private fun handleClose(request: NativeBridgeRequest): NativeBridgeResponse {
        val overlayId = request.payload["overlayId"] as? String
            ?: return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "OVERLAY_INVALID_REQUEST",
                    message = "overlayId is required",
                ),
            )

        val removed = activeOverlays.remove(overlayId)
        return if (removed != null) {
            generation.incrementAndGet()
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_SUCCESS,
                result = mapOf(
                    "overlayId" to overlayId,
                    "closed" to true,
                    "generation" to generation.get(),
                ),
            )
        } else {
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "OVERLAY_NOT_FOUND",
                    message = "overlay not found: $overlayId",
                ),
            )
        }
    }

    private fun handleList(request: NativeBridgeRequest): NativeBridgeResponse {
        val overlays = activeOverlays.values.map { info ->
            mapOf(
                "overlayId" to info.overlayId,
                "visible" to info.visible,
                "width" to info.width,
                "height" to info.height,
                "x" to info.x,
                "y" to info.y,
                "generation" to info.generation,
            )
        }
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "overlays" to overlays,
                "count" to overlays.size,
                "generation" to generation.get(),
            ),
        )
    }

    private fun handleCloseAll(request: NativeBridgeRequest): NativeBridgeResponse {
        val count = activeOverlays.size
        activeOverlays.clear()
        generation.incrementAndGet()
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "closed" to count,
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
                message = "unknown overlay operation: ${request.operation}",
            ),
        )
    }

    companion object {
        const val OP_STATUS = "overlay.status"
        const val OP_REQUEST_PERMISSION = "overlay.request_permission"
        const val OP_CREATE = "overlay.create"
        const val OP_UPDATE = "overlay.update"
        const val OP_SHOW = "overlay.show"
        const val OP_HIDE = "overlay.hide"
        const val OP_CLOSE = "overlay.close"
        const val OP_LIST = "overlay.list"
        const val OP_CLOSE_ALL = "overlay.close_all"
    }
}
