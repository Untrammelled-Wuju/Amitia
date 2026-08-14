package com.amitia.amitia_app.nativeprovider

import android.content.Context
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeHealth
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import java.util.concurrent.atomic.AtomicLong
import java.util.concurrent.atomic.AtomicReference

internal class AndroidNativeHost private constructor(
    private val context: Context,
) {

    private val handlers = LinkedHashMap<String, AndroidNativeOperationHandler>()
    private val hostGeneration = AtomicLong(1L)
    private val foregroundRef = AtomicReference(true)
    private val handlerMutex = Mutex()
    private val capabilityCache = AtomicReference<Map<String, Boolean>>(emptyMap())

    val currentGeneration: Long
        get() = hostGeneration.get()

    val foreground: Boolean
        get() = foregroundRef.get()

    suspend fun registerHandler(handler: AndroidNativeOperationHandler) {
        handlerMutex.withLock {
            val supportedOperations = mutableListOf<String>()
            for ((existingOp, existingHandler) in handlers) {
                if (existingHandler === handler) {
                    supportedOperations.add(existingOp)
                }
            }
            for (op in supportedOperations) {
                handlers.remove(op)
            }
            val allOperations = collectOperations(handler)
            for (op in allOperations) {
                if (handlers.containsKey(op)) {
                    throw IllegalStateException("AndroidNativeHost: duplicate handler registration for operation $op")
                }
                handlers[op] = handler
            }
            capabilityCache.clear()
        }
    }

    private fun collectOperations(handler: AndroidNativeOperationHandler): List<String> {
        val knownOperations = listOf(
            "accessibility.status", "accessibility.open_settings",
            "clipboard.status", "clipboard.read_text", "clipboard.write_text", "clipboard.clear",
            "share.status", "share.send",
            "notification.status", "notification.list", "notification.get",
            "notification.post", "notification.cancel_own", "notification.dismiss",
            "notification.open", "notification.invoke_action",
            "root.status", "root.request", "root.execute",
            "uitree.snapshot", "uitree.query",
            "interaction.tap", "interaction.swipe", "interaction.input_text",
            "interaction.clear_text", "interaction.scroll", "interaction.action",
            "display.list", "display.get", "display.primary",
            "virtualdisplay.create", "virtualdisplay.resize", "virtualdisplay.release",
            "camera.status", "camera.list", "camera.capture",
            "overlay.status", "overlay.request_permission", "overlay.create",
            "overlay.update", "overlay.show", "overlay.hide", "overlay.close",
            "overlay.list", "overlay.close_all",
            "externalautomation.resolve_app", "externalautomation.open_app",
            "externalautomation.resolve_uri", "externalautomation.open_uri",
            "externalautomation.open_settings", "externalautomation.invoke_intent",
            "externalautomation.foreground_state", "externalautomation.wait_foreground",
        )
        return knownOperations.filter { handler.supports(it) }
    }

    suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse {
        if (request.platform != NativeBridgeProtocol.PLATFORM_ANDROID) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = NativeBridgeProtocol.ERR_INVALID_PLATFORM,
                    message = "unsupported platform: ${request.platform}",
                ),
            )
        }

        if (request.protocolVersion != NativeBridgeProtocol.PROTOCOL_VERSION) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = NativeBridgeProtocol.ERR_UNSUPPORTED_PROTOCOL,
                    message = "unsupported protocol version: ${request.protocolVersion}",
                ),
            )
        }

        if (request.requestId.isBlank()) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = NativeBridgeProtocol.ERR_INVALID_REQUEST,
                    message = "requestId is required",
                ),
            )
        }

        val handler = handlerMutex.withLock { handlers[request.operation] }
            ?: return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = NativeBridgeProtocol.ERR_OPERATION_NOT_SUPPORTED,
                    message = "operation not supported: ${request.operation}",
                ),
            )

        return try {
            handler.execute(request)
        } catch (e: Exception) {
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = NativeBridgeProtocol.ERR_INTERNAL,
                    message = "internal error: ${e.message}",
                ),
            )
        }
    }

    fun health(): NativeBridgeHealth {
        val capabilities = buildCapabilities()
        return NativeBridgeHealth(
            status = NativeBridgeProtocol.HEALTH_READY,
            platform = NativeBridgeProtocol.PLATFORM_ANDROID,
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            hostGeneration = hostGeneration.get(),
            foreground = foregroundRef.get(),
            capabilities = capabilities,
        )
    }

    private fun buildCapabilities(): Map<String, Boolean> {
        capabilityCache.get()?.let { return it }
        val snapshot = handlerMutex.let {
            LinkedHashMap(handlers.keys)
        }
        val capabilities = linkedMapOf<String, Boolean>()
        for (op in snapshot) {
            capabilities[op] = true
        }
        capabilityCache.compareAndSet(capabilities, capabilities)
        return capabilities
    }

    fun didEnterBackground() {
        foregroundRef.set(false)
        hostGeneration.incrementAndGet()
    }

    fun willEnterForeground() {
        foregroundRef.set(true)
        hostGeneration.incrementAndGet()
    }

    fun hostInvalidated() {
        hostGeneration.incrementAndGet()
    }

    companion object {
        @Volatile
        private var instance: AndroidNativeHost? = null

        fun shared(context: Context): AndroidNativeHost {
            return instance ?: synchronized(this) {
                instance ?: AndroidNativeHost(context.applicationContext).also { instance = it }
            }
        }
    }
}
