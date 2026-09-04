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
            capabilityCache.set(handlers.keys.associateWith { true })
            hostGeneration.incrementAndGet()
        }
    }

    private fun collectOperations(handler: AndroidNativeOperationHandler): List<String> {
        return handler.operations.toList()
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
        // registerHandler publishes an immutable capability snapshot while the
        // handler mutex is held. Health reads only that atomic snapshot, so it
        // never iterates the mutable handler map concurrently with registration.
        val capabilities = capabilityCache.get()
        val status = if (capabilities.isEmpty()) {
            NativeBridgeProtocol.HEALTH_UNKNOWN
        } else {
            NativeBridgeProtocol.HEALTH_READY
        }
        return NativeBridgeHealth(
            status = status,
            platform = NativeBridgeProtocol.PLATFORM_ANDROID,
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            hostGeneration = hostGeneration.get(),
            foreground = foregroundRef.get(),
            capabilities = capabilities,
        )
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
