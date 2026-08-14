package com.amitia.amitia_app.nativeprovider

import android.content.Context
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import io.flutter.embedding.engine.plugins.FlutterPlugin
import io.flutter.plugin.common.MethodCall
import io.flutter.plugin.common.MethodChannel
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

class AndroidNativeBridgePlugin : FlutterPlugin {

    private var methodChannel: MethodChannel? = null
    private var host: AndroidNativeHost? = null
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main)

    override fun onAttachedToEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        val context: Context = binding.applicationContext
        host = AndroidNativeHost.shared(context)

        val methodChannel = MethodChannel(binding.binaryMessenger, CHANNEL_NAME)
        methodChannel.setMethodCallHandler { call, result ->
            handleMethodCall(call, result)
        }
        this.methodChannel = methodChannel
    }

    override fun onDetachedFromEngine(binding: FlutterPlugin.FlutterPluginBinding) {
        methodChannel?.setMethodCallHandler(null)
        methodChannel = null
        host = null
    }

    private fun handleMethodCall(call: MethodCall, result: MethodChannel.Result) {
        when (call.method) {
            METHOD_HEALTH -> {
                val currentHost = host
                if (currentHost == null) {
                    result.error(
                        NativeBridgeProtocol.ERR_HOST_UNAVAILABLE,
                        "Android Native Host not available",
                        null,
                    )
                    return
                }
                val health = currentHost.health()
                result.success(healthToMap(health))
            }

            METHOD_EXECUTE -> {
                val currentHost = host
                if (currentHost == null) {
                    result.error(
                        NativeBridgeProtocol.ERR_HOST_UNAVAILABLE,
                        "Android Native Host not available",
                        null,
                    )
                    return
                }
                val requestArg = call.arguments as? Map<*, *>
                if (requestArg == null) {
                    result.error(
                        NativeBridgeProtocol.ERR_INVALID_REQUEST,
                        "invalid request payload",
                        null,
                    )
                    return
                }
                val request = parseRequest(requestArg)
                scope.launch {
                    try {
                        val response = withContext(Dispatchers.Default) {
                            currentHost.execute(request)
                        }
                        result.success(responseToMap(response))
                    } catch (e: Exception) {
                        result.error(
                            NativeBridgeProtocol.ERR_INTERNAL,
                            "internal error: ${e.message}",
                            null,
                        )
                    }
                }
            }

            else -> result.notImplemented()
        }
    }

    private fun parseRequest(map: Map<*, *>): com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest {
        val protocolVersion = (map["protocolVersion"] as? Number)?.toInt()
            ?: NativeBridgeProtocol.PROTOCOL_VERSION
        val requestId = (map["requestId"] as? String).orEmpty()
        val platform = (map["platform"] as? String).orEmpty()
        val operation = (map["operation"] as? String).orEmpty()
        val payload = (map["payload"] as? Map<String, Any?>) ?: emptyMap()
        return com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest(
            protocolVersion = protocolVersion,
            requestId = requestId,
            platform = platform,
            operation = operation,
            payload = payload,
        )
    }

    private fun responseToMap(response: com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse): Map<String, Any?> {
        val result = linkedMapOf<String, Any?>()
        result["protocolVersion"] = response.protocolVersion
        result["requestId"] = response.requestId
        result["status"] = response.status
        response.result?.let { result["result"] = it }
        response.error?.let {
            val errorMap = linkedMapOf<String, Any?>()
            errorMap["code"] = it.code
            errorMap["message"] = it.message
            it.domainCode?.let { dc -> errorMap["domainCode"] = dc }
            result["error"] = errorMap
        }
        return result
    }

    private fun healthToMap(health: com.amitia.amitia_app.nativeprovider.model.NativeBridgeHealth): Map<String, Any?> {
        return mapOf(
            "status" to health.status,
            "platform" to health.platform,
            "protocolVersion" to health.protocolVersion,
            "hostGeneration" to health.hostGeneration,
            "foreground" to health.foreground,
            "capabilities" to health.capabilities,
        )
    }

    companion object {
        const val CHANNEL_NAME = "com.amitia.android_native/bridge"
        const val METHOD_HEALTH = "nativeBridge.health"
        const val METHOD_EXECUTE = "nativeBridge.execute"
    }
}
