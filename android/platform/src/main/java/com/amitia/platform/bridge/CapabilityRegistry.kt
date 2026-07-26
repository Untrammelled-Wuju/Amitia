package com.amitia.platform.bridge

import com.amitia.platform.permissions.PermissionBroker
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class CapabilityRegistry @Inject constructor(
    private val permissionBroker: PermissionBroker,
    private val providers: Map<String, CapabilityProvider>
) {

    fun listActions(): List<String> = providers.keys.toList()

    fun hasAction(action: String): Boolean = providers.containsKey(action)

    suspend fun execute(request: NativeActionRequest): NativeActionResult {
        val provider = providers[request.action]
            ?: return NativeActionResult.Failed("unsupported_action: ${request.action}")

        val requiredPermission = request.requiresPermission ?: provider.requiredPermission()
        if (!requiredPermission.isNullOrBlank()) {
            val granted = permissionBroker.isGranted(requiredPermission)
            if (!granted) {
                val result = permissionBroker.request(requiredPermission)
                if (result !is PermissionBroker.PermissionResult.Granted) {
                    return NativeActionResult.Denied(requiredPermission, "permission denied")
                }
            }
        }
        return runCatching { provider.execute(request) }
            .getOrElse { NativeActionResult.Failed(it.message ?: "execute_failed", it) }
    }
}

interface CapabilityProvider {

    fun action(): String

    fun requiredPermission(): String?

    suspend fun execute(request: NativeActionRequest): NativeActionResult
}
