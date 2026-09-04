package com.amitia.amitia_app.nativeprovider.notification

import android.content.Context
import android.content.pm.PackageManager
import android.service.notification.StatusBarNotification

internal class NotificationNativeHandler(context: Context) {

    private val appContext = context.applicationContext
    private val stateReader = NotificationStateReader(appContext)
    private val projection = NotificationProjectionInternal()
    private val poster = NotificationPoster(appContext)
    private val actionExecutor = NotificationActionExecutor()

    fun execute(request: NativeNotificationRequest): NativeNotificationResponse {
        return when (request.operation) {
            OP_STATUS -> handleStatus(request)
            OP_LIST -> handleList(request)
            OP_GET -> handleGet(request)
            OP_POST -> handlePost(request)
            OP_CANCEL_OWN -> handleCancelOwn(request)
            OP_DISMISS -> handleDismiss(request)
            OP_OPEN -> handleOpen(request)
            OP_INVOKE_ACTION -> handleInvokeAction(request)
            else -> NativeNotificationResponse(
                requestId = request.requestId,
                status = "error",
                error = NativeNotificationError(
                    code = "OPERATION_NOT_SUPPORTED",
                    message = "unknown notification operation: ${request.operation}",
                ),
            )
        }
    }

    fun onListenerConnected() {
        projection.bumpGeneration()
    }

    fun onListenerDisconnected() {
        projection.bumpGeneration()
    }

    fun currentGeneration(): Long = projection.currentGeneration()

    private fun handleStatus(request: NativeNotificationRequest): NativeNotificationResponse {
        val state = stateReader.readState()
        val result = mapOf(
            "supported" to state.supported,
            "listenerDeclared" to state.listenerDeclared,
            "listenerGranted" to state.listenerGranted,
            "listenerConnected" to state.listenerConnected,
            "postPermissionRequired" to state.postPermissionRequired,
            "postPermissionGranted" to state.postPermissionGranted,
            "notificationsEnabled" to state.notificationsEnabled,
            "canRead" to state.canRead,
            "canDismiss" to state.canDismiss,
            "canPost" to state.canPost,
            "userActionRequired" to state.userActionRequired,
            "state" to state.state,
        )
        return NativeNotificationResponse(
            requestId = request.requestId,
            status = "success",
            result = result,
        )
    }

    private fun handleList(request: NativeNotificationRequest): NativeNotificationResponse {
        val state = stateReader.readState()
        if (!state.canRead) {
            return if (!state.listenerGranted) {
                permissionRequired(request.requestId)
            } else {
                notConnected(request.requestId)
            }
        }

        val service = NotificationServiceRegistry.current()
            ?: return notConnected(request.requestId)

        val limit = (request.payload["limit"] as? Number)?.toInt() ?: 50
        val actualLimit = limit.coerceIn(1, 100)
        val packageNameFilter = request.payload["packageName"] as? String
        val includeOngoing = request.payload["includeOngoing"] as? Boolean ?: false
        val includeOwn = request.payload["includeOwn"] as? Boolean ?: false

        val activeNotifications = service.activeNotifications ?: emptyArray()

        val notifications = mutableListOf<NotificationProjection>()
        var filteredCount = 0

        for (sbn in activeNotifications) {
            if (!includeOwn && projection.isRuntimeService(sbn.packageName, sbn.notification.channelId)) {
                filteredCount++
                continue
            }
            if (packageNameFilter != null && sbn.packageName != packageNameFilter) {
                filteredCount++
                continue
            }
            if (!includeOngoing && sbn.isOngoing) {
                filteredCount++
                continue
            }
            val appLabel = resolveAppLabel(sbn.packageName)
            notifications.add(projection.project(sbn, appLabel, projection.currentGeneration()))
            if (notifications.size >= actualLimit) break
        }

        notifications.sortByDescending { it.postedAt }

        val result = mapOf(
            "notifications" to notifications,
            "count" to notifications.size,
            "filteredCount" to filteredCount,
        )

        return NativeNotificationResponse(
            requestId = request.requestId,
            status = "success",
            result = result,
        )
    }

    private fun handleGet(request: NativeNotificationRequest): NativeNotificationResponse {
        val ref = request.payload["notificationRef"] as? String
            ?: return notFound(request.requestId)

        val service = NotificationServiceRegistry.current()
            ?: return notConnected(request.requestId)

        val activeNotifications = service.activeNotifications ?: emptyArray()
        val target = activeNotifications.firstOrNull {
            projection.assignRef(it.key) == ref || projection.lookupKey(ref) == it.key
        }

        if (target == null) {
            return NativeNotificationResponse(
                requestId = request.requestId,
                status = "error",
                error = NativeNotificationError(
                    code = "NOTIFICATION_NOT_FOUND",
                    message = "notification not found or stale",
                ),
            )
        }

        val appLabel = resolveAppLabel(target.packageName)
        val projected = projection.project(target, appLabel, projection.currentGeneration())

        return NativeNotificationResponse(
            requestId = request.requestId,
            status = "success",
            result = mapOf("notification" to projected),
        )
    }

    private fun handlePost(request: NativeNotificationRequest): NativeNotificationResponse {
        val title = (request.payload["title"] as? String) ?: ""
        val body = (request.payload["body"] as? String) ?: ""
        val channel = (request.payload["channel"] as? String) ?: "amitia_agent"
        val silent = request.payload["silent"] as? Boolean ?: false

        if (title.isBlank() && body.isBlank()) {
            return NativeNotificationResponse(
                requestId = request.requestId,
                status = "error",
                error = NativeNotificationError(
                    code = "NOTIFICATION_POST_FAILED",
                    message = "both title and body are empty",
                ),
            )
        }

        val state = stateReader.readState()
        if (!state.canPost) {
            return if (state.notificationsEnabled) {
                NativeNotificationResponse(
                    requestId = request.requestId,
                    status = "error",
                    error = NativeNotificationError(
                        code = "NOTIFICATION_POST_PERMISSION_REQUIRED",
                        message = "POST_NOTIFICATIONS not granted",
                    ),
                )
            } else {
                NativeNotificationResponse(
                    requestId = request.requestId,
                    status = "error",
                    error = NativeNotificationError(
                        code = "NOTIFICATION_POST_DISABLED",
                        message = "notifications disabled for this app",
                    ),
                )
            }
        }

        val ref = poster.post(
            title = title.take(256),
            body = body.take(4096),
            channel = channel,
            silent = silent,
        )

        return NativeNotificationResponse(
            requestId = request.requestId,
            status = "success",
            result = mapOf("notificationRef" to ref, "posted" to true),
        )
    }

    private fun handleCancelOwn(request: NativeNotificationRequest): NativeNotificationResponse {
        val ref = request.payload["notificationRef"] as? String
            ?: return NativeNotificationResponse(
                requestId = request.requestId,
                status = "error",
                error = NativeNotificationError(
                    code = "NOTIFICATION_CANCEL_FAILED",
                    message = "notificationRef is required",
                ),
            )

        val cancelled = poster.cancelOwn(ref)

        return if (cancelled) {
            NativeNotificationResponse(
                requestId = request.requestId,
                status = "success",
                result = mapOf("cancelled" to true),
            )
        } else {
            NativeNotificationResponse(
                requestId = request.requestId,
                status = "error",
                error = NativeNotificationError(
                    code = "NOTIFICATION_NOT_FOUND",
                    message = "own notification not found",
                ),
            )
        }
    }

    private fun handleDismiss(request: NativeNotificationRequest): NativeNotificationResponse {
        val state = stateReader.readState()
        if (!state.canDismiss) {
            return permissionRequired(request.requestId)
        }

        val ref = request.payload["notificationRef"] as? String
            ?: return notFound(request.requestId)

        val service = NotificationServiceRegistry.current()
            ?: return notConnected(request.requestId)

        val key = projection.lookupKey(ref)
        val activeNotifications = service.activeNotifications ?: emptyArray()
        val target = activeNotifications.firstOrNull {
            if (key != null) it.key == key else projection.assignRef(it.key) == ref
        } ?: return notFound(request.requestId)

        if (!target.isClearable) {
            return NativeNotificationResponse(
                requestId = request.requestId,
                status = "error",
                error = NativeNotificationError(
                    code = "NOTIFICATION_NOT_DISMISSIBLE",
                    message = "notification is not dismissible",
                ),
            )
        }

        service.cancelNotification(target.key)

        return NativeNotificationResponse(
            requestId = request.requestId,
            status = "success",
            result = mapOf("requested" to true, "dismissed" to null),
        )
    }

    private fun handleOpen(request: NativeNotificationRequest): NativeNotificationResponse {
        val state = stateReader.readState()
        if (!state.canDismiss) {
            return permissionRequired(request.requestId)
        }

        val ref = request.payload["notificationRef"] as? String
            ?: return notFound(request.requestId)

        val service = NotificationServiceRegistry.current()
            ?: return notConnected(request.requestId)

        val key = projection.lookupKey(ref)
        val activeNotifications = service.activeNotifications ?: emptyArray()
        val target = activeNotifications.firstOrNull {
            if (key != null) it.key == key else projection.assignRef(it.key) == ref
        } ?: return notFound(request.requestId)

        if (target.notification.contentIntent == null) {
            return NativeNotificationResponse(
                requestId = request.requestId,
                status = "error",
                error = NativeNotificationError(
                    code = "NOTIFICATION_CONTENT_ACTION_UNAVAILABLE",
                    message = "notification has no content action",
                ),
            )
        }

        val invoked = actionExecutor.openContentIntent(target)

        return NativeNotificationResponse(
            requestId = request.requestId,
            status = "success",
            result = mapOf("invoked" to invoked),
        )
    }

    private fun handleInvokeAction(request: NativeNotificationRequest): NativeNotificationResponse {
        val state = stateReader.readState()
        if (!state.canDismiss) {
            return permissionRequired(request.requestId)
        }

        val ref = request.payload["notificationRef"] as? String
        val actionRef = request.payload["actionRef"] as? String

        if (ref.isNullOrBlank() || actionRef.isNullOrBlank()) {
            return NativeNotificationResponse(
                requestId = request.requestId,
                status = "error",
                error = NativeNotificationError(
                    code = "NOTIFICATION_ACTION_NOT_FOUND",
                    message = "notificationRef and actionRef are required",
                ),
            )
        }

        val service = NotificationServiceRegistry.current()
            ?: return notConnected(request.requestId)

        val key = projection.lookupKey(ref)
        val activeNotifications = service.activeNotifications ?: emptyArray()
        val target = activeNotifications.firstOrNull {
            if (key != null) it.key == key else projection.assignRef(it.key) == ref
        } ?: return NativeNotificationResponse(
            requestId = request.requestId,
            status = "error",
            error = NativeNotificationError(
                code = "NOTIFICATION_ACTION_STALE",
                message = "notification reference is stale",
            ),
        )

        val invoked = actionExecutor.executeAction(target, actionRef, projection)

        return if (invoked) {
            NativeNotificationResponse(
                requestId = request.requestId,
                status = "success",
                result = mapOf("invoked" to true),
            )
        } else {
            NativeNotificationResponse(
                requestId = request.requestId,
                status = "error",
                error = NativeNotificationError(
                    code = "NOTIFICATION_ACTION_FAILED",
                    message = "failed to invoke notification action",
                ),
            )
        }
    }

    private fun resolveAppLabel(packageName: String): String {
        return try {
            val pm = appContext.packageManager
            val appInfo = pm.getApplicationInfo(packageName, 0)
            pm.getApplicationLabel(appInfo).toString()
        } catch (e: PackageManager.NameNotFoundException) {
            packageName
        }
    }

    private fun permissionRequired(requestId: String) = NativeNotificationResponse(
        requestId = requestId,
        status = "error",
        error = NativeNotificationError(
            code = "NOTIFICATION_LISTENER_PERMISSION_REQUIRED",
            message = "notification listener access not granted",
        ),
    )

    private fun notConnected(requestId: String) = NativeNotificationResponse(
        requestId = requestId,
        status = "error",
        error = NativeNotificationError(
            code = "NOTIFICATION_LISTENER_NOT_CONNECTED",
            message = "notification listener not connected",
        ),
    )

    private fun notFound(requestId: String) = NativeNotificationResponse(
        requestId = requestId,
        status = "error",
        error = NativeNotificationError(
            code = "NOTIFICATION_NOT_FOUND",
            message = "notification not found",
        ),
    )

    companion object {
        const val OP_STATUS = "notification.status"
        const val OP_LIST = "notification.list"
        const val OP_GET = "notification.get"
        const val OP_POST = "notification.post"
        const val OP_CANCEL_OWN = "notification.cancel_own"
        const val OP_DISMISS = "notification.dismiss"
        const val OP_OPEN = "notification.open"
        const val OP_INVOKE_ACTION = "notification.invoke_action"
    }
}
