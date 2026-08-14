package com.amitia.amitia_app.nativeprovider.externalautomation

import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.provider.Settings
import com.amitia.amitia_app.nativeprovider.AndroidNativeOperationHandler
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeError
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeProtocol
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeRequest
import com.amitia.amitia_app.nativeprovider.model.NativeBridgeResponse
import java.util.concurrent.atomic.AtomicLong

internal class ExternalAutomationNativeHandler(
    private val context: Context,
) : AndroidNativeOperationHandler {

    private val generation = AtomicLong(0L)

    override val operations: Set<String> = setOf(
        OP_STATUS,
        OP_RESOLVE_APP,
        OP_OPEN_APP,
        OP_RESOLVE_URI,
        OP_OPEN_URI,
        OP_OPEN_SETTINGS,
        OP_INVOKE_INTENT,
        OP_FOREGROUND_STATE,
        OP_WAIT_FOREGROUND,
    )

    override suspend fun execute(request: NativeBridgeRequest): NativeBridgeResponse {
        return when (request.operation) {
            OP_STATUS -> handleStatus(request)
            OP_RESOLVE_APP -> handleResolveApp(request)
            OP_OPEN_APP -> handleOpenApp(request)
            OP_RESOLVE_URI -> handleResolveUri(request)
            OP_OPEN_URI -> handleOpenUri(request)
            OP_OPEN_SETTINGS -> handleOpenSettings(request)
            OP_INVOKE_INTENT -> handleInvokeIntent(request)
            OP_FOREGROUND_STATE -> handleForegroundState(request)
            OP_WAIT_FOREGROUND -> handleWaitForeground(request)
            else -> unsupportedOperation(request)
        }
    }

    private fun handleResolveApp(request: NativeBridgeRequest): NativeBridgeResponse {
        val query = request.payload["query"] as? String ?: ""
        val byPackage = request.payload["byPackage"] as? Boolean ?: true
        val byLabel = request.payload["byLabel"] as? Boolean ?: true

        if (query.isBlank()) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "EXTERNAL_AUTOMATION_INVALID_REQUEST",
                    message = "query is required",
                ),
            )
        }

        return try {
            val pm = context.packageManager
            val resolved = if (byPackage) {
                try {
                    val info = pm.getApplicationInfo(query, 0)
                    val label = pm.getApplicationLabel(info).toString()
                    listOf(
                        mapOf(
                            "packageName" to query,
                            "label" to label,
                            "mainActivity" to null,
                            "installed" to true,
                        ),
                    )
                } catch (_: PackageManager.NameNotFoundException) {
                    if (byLabel) searchByLabel(pm, query) else emptyList()
                }
            } else if (byLabel) {
                searchByLabel(pm, query)
            } else {
                emptyList()
            }
            generation.incrementAndGet()
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_SUCCESS,
                result = mapOf(
                    "apps" to resolved,
                    "count" to resolved.size,
                    "generation" to generation.get(),
                ),
            )
        } catch (e: Exception) {
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "EXTERNAL_AUTOMATION_RESOLVE_FAILED",
                    message = "resolve app failed: ${e.message}",
                ),
            )
        }
    }

    private fun searchByLabel(pm: PackageManager, query: String): List<Map<String, Any?>> {
        return try {
            val intent = Intent(Intent.ACTION_MAIN, null).apply {
                addCategory(Intent.CATEGORY_LAUNCHER)
            }
            val resolveInfos = pm.queryIntentActivities(intent, 0)
            resolveInfos.filter { info ->
                info.loadLabel(pm).toString().contains(query, ignoreCase = true)
            }.take(10).map { info ->
                mapOf(
                    "packageName" to info.activityInfo.packageName,
                    "label" to info.loadLabel(pm).toString(),
                    "mainActivity" to info.activityInfo.name,
                    "installed" to true,
                )
            }
        } catch (_: Exception) {
            emptyList()
        }
    }

    private fun handleOpenApp(request: NativeBridgeRequest): NativeBridgeResponse {
        val packageName = request.payload["packageName"] as? String ?: ""
        val activityName = request.payload["activityName"] as? String
        val bringToFront = request.payload["bringToFront"] as? Boolean ?: true

        if (packageName.isBlank()) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "EXTERNAL_AUTOMATION_INVALID_REQUEST",
                    message = "packageName is required",
                ),
            )
        }

        return try {
            val intent = if (activityName != null) {
                Intent().apply {
                    setClassName(packageName, activityName)
                    if (bringToFront) addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                }
            } else {
                context.packageManager.getLaunchIntentForPackage(packageName)?.apply {
                    if (bringToFront) addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                }
            }

            if (intent == null) {
                return NativeBridgeResponse(
                    protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                    requestId = request.requestId,
                    status = NativeBridgeProtocol.STATUS_ERROR,
                    error = NativeBridgeError(
                        code = "EXTERNAL_AUTOMATION_APP_NOT_FOUND",
                        message = "cannot resolve launch intent for: $packageName",
                    ),
                )
            }

            context.startActivity(intent)
            generation.incrementAndGet()
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_SUCCESS,
                result = mapOf(
                    "launched" to true,
                    "packageName" to packageName,
                    "generation" to generation.get(),
                ),
            )
        } catch (e: Exception) {
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "EXTERNAL_AUTOMATION_LAUNCH_FAILED",
                    message = "open app failed: ${e.message}",
                ),
            )
        }
    }

    private fun handleResolveUri(request: NativeBridgeRequest): NativeBridgeResponse {
        val uri = request.payload["uri"] as? String ?: ""
        if (uri.isBlank()) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "EXTERNAL_AUTOMATION_INVALID_REQUEST",
                    message = "uri is required",
                ),
            )
        }

        return try {
            val pm = context.packageManager
            val intent = Intent(Intent.ACTION_VIEW, Uri.parse(uri))
            val resolveInfos = pm.queryIntentActivities(intent, 0)
            val resolved = resolveInfos.map { info ->
                mapOf(
                    "packageName" to info.activityInfo.packageName,
                    "label" to info.loadLabel(pm).toString(),
                    "activityName" to info.activityInfo.name,
                )
            }
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_SUCCESS,
                result = mapOf(
                    "uri" to uri,
                    "handlers" to resolved,
                    "count" to resolved.size,
                ),
            )
        } catch (e: Exception) {
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "EXTERNAL_AUTOMATION_RESOLVE_FAILED",
                    message = "resolve URI failed: ${e.message}",
                ),
            )
        }
    }

    private fun handleOpenUri(request: NativeBridgeRequest): NativeBridgeResponse {
        val uri = request.payload["uri"] as? String ?: ""
        if (uri.isBlank()) {
            return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "EXTERNAL_AUTOMATION_INVALID_REQUEST",
                    message = "uri is required",
                ),
            )
        }

        return try {
            val intent = Intent(Intent.ACTION_VIEW, Uri.parse(uri)).apply {
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            }
            context.startActivity(intent)
            generation.incrementAndGet()
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_SUCCESS,
                result = mapOf(
                    "opened" to true,
                    "uri" to uri,
                    "generation" to generation.get(),
                ),
            )
        } catch (e: Exception) {
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "EXTERNAL_AUTOMATION_OPEN_FAILED",
                    message = "open URI failed: ${e.message}",
                ),
            )
        }
    }

    private fun handleOpenSettings(request: NativeBridgeRequest): NativeBridgeResponse {
        val packageName = request.payload["packageName"] as? String
        val action = request.payload["action"] as? String

        return try {
            val intent = when {
                action != null -> Intent(action).apply { addFlags(Intent.FLAG_ACTIVITY_NEW_TASK) }
                packageName != null -> Intent(Settings.ACTION_APPLICATION_DETAILS_SETTINGS).apply {
                    data = Uri.parse("package:$packageName")
                    addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                }
                else -> Intent(Settings.ACTION_SETTINGS).apply {
                    addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                }
            }
            context.startActivity(intent)
            generation.incrementAndGet()
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_SUCCESS,
                result = mapOf(
                    "opened" to true,
                    "generation" to generation.get(),
                ),
            )
        } catch (e: Exception) {
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "EXTERNAL_AUTOMATION_SETTINGS_FAILED",
                    message = "open settings failed: ${e.message}",
                ),
            )
        }
    }

    private fun handleInvokeIntent(request: NativeBridgeRequest): NativeBridgeResponse {
        val action = request.payload["action"] as? String
            ?: return NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "EXTERNAL_AUTOMATION_INVALID_REQUEST",
                    message = "action is required",
                ),
            )

        return try {
            val intent = Intent(action).apply {
                addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            }
            request.payload["data"]?.let { data ->
                if (data is String) this.data = Uri.parse(data)
            }
            request.payload["packageName"]?.let { pkg ->
                if (pkg is String) setPackage(pkg)
            }
            request.payload["componentName"]?.let { comp ->
                if (comp is String) {
                    request.payload["packageName"]?.let { pkg ->
                        if (pkg is String) setClassName(pkg, comp)
                    }
                }
            }
            request.payload["categories"]?.let { cats ->
                if (cats is List<*>) {
                    cats.forEach { cat ->
                        if (cat is String) addCategory(cat)
                    }
                }
            }
            context.startActivity(intent)
            generation.incrementAndGet()
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_SUCCESS,
                result = mapOf(
                    "invoked" to true,
                    "action" to action,
                    "generation" to generation.get(),
                ),
            )
        } catch (e: Exception) {
            NativeBridgeResponse(
                protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
                requestId = request.requestId,
                status = NativeBridgeProtocol.STATUS_ERROR,
                error = NativeBridgeError(
                    code = "EXTERNAL_AUTOMATION_INTENT_FAILED",
                    message = "invoke intent failed: ${e.message}",
                ),
            )
        }
    }

    private fun handleStatus(request: NativeBridgeRequest): NativeBridgeResponse {
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "generation" to generation.get(),
                "capabilities" to listOf("resolve_app", "open_app", "resolve_uri", "open_uri", "open_settings", "invoke_intent"),
            ),
        )
    }

    private fun handleForegroundState(request: NativeBridgeRequest): NativeBridgeResponse {
        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_SUCCESS,
            result = mapOf(
                "foregroundPackageName" to "",
                "foregroundActivityName" to null,
                "isForeground" to false,
                "state" to "unavailable",
                "reason" to "foreground state detection requires accessibility service",
            ),
        )
    }

    private fun handleWaitForeground(request: NativeBridgeRequest): NativeBridgeResponse {
        val targetPackage = request.payload["packageName"] as? String ?: ""
        val timeoutMs = (request.payload["timeoutMs"] as? Number)?.toLong() ?: 5000L

        return NativeBridgeResponse(
            protocolVersion = NativeBridgeProtocol.PROTOCOL_VERSION,
            requestId = request.requestId,
            status = NativeBridgeProtocol.STATUS_ERROR,
            error = NativeBridgeError(
                code = "EXTERNAL_AUTOMATION_NOT_IMPLEMENTED",
                message = "wait foreground requires accessibility service integration",
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
                message = "unknown external automation operation: ${request.operation}",
            ),
        )
    }

    companion object {
        const val OP_STATUS = "external_automation.status"
        const val OP_RESOLVE_APP = "external_automation.resolve_app"
        const val OP_OPEN_APP = "external_automation.open_app"
        const val OP_RESOLVE_URI = "external_automation.resolve_uri"
        const val OP_OPEN_URI = "external_automation.open_uri"
        const val OP_OPEN_SETTINGS = "external_automation.open_settings"
        const val OP_INVOKE_INTENT = "external_automation.invoke_intent"
        const val OP_FOREGROUND_STATE = "external_automation.foreground_state"
        const val OP_WAIT_FOREGROUND = "external_automation.wait_foreground"
    }
}
