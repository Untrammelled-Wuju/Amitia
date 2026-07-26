package com.amitia.platform.permissions

import kotlinx.coroutines.flow.Flow

interface PermissionBroker {

    suspend fun request(permission: String): PermissionResult

    suspend fun requestMultiple(permissions: List<String>): Map<String, PermissionResult>

    suspend fun shouldShowRationale(permission: String): Boolean

    suspend fun isGranted(permission: String): Boolean

    suspend fun openAppSettings(): Boolean

    fun observePermissionChanges(): Flow<Map<String, PermissionResult>>

    object Permissions {
        const val INTERNET = "android.permission.INTERNET"
        const val ACCESS_NETWORK_STATE = "android.permission.ACCESS_NETWORK_STATE"
        const val POST_NOTIFICATIONS = "android.permission.POST_NOTIFICATIONS"
        const val RECORD_AUDIO = "android.permission.RECORD_AUDIO"
        const val CAMERA = "android.permission.CAMERA"
        const val READ_MEDIA_IMAGES = "android.permission.READ_MEDIA_IMAGES"
        const val READ_MEDIA_AUDIO = "android.permission.READ_MEDIA_AUDIO"
        const val READ_MEDIA_VIDEO = "android.permission.READ_MEDIA_VIDEO"
        const val WRITE_EXTERNAL_STORAGE = "android.permission.WRITE_EXTERNAL_STORAGE"
        const val READ_EXTERNAL_STORAGE = "android.permission.READ_EXTERNAL_STORAGE"
        const val FOREGROUND_SERVICE = "android.permission.FOREGROUND_SERVICE"
        const val FOREGROUND_SERVICE_DATA_SYNC = "android.permission.FOREGROUND_SERVICE_DATA_SYNC"
        const val WAKE_LOCK = "android.permission.WAKE_LOCK"
        const val RECEIVE_BOOT_COMPLETED = "android.permission.RECEIVE_BOOT_COMPLETED"
        const val VIBRATE = "android.permission.VIBRATE"
    }

    sealed class PermissionResult {
        object Granted : PermissionResult()
        object Denied : PermissionResult()
        object PermanentlyDenied : PermissionResult()
        data class RationaleRequired(val message: String) : PermissionResult()
    }

    data class PermissionGroup(
        val id: String,
        val name: String,
        val description: String,
        val permissions: List<String>,
        val rationaleMessage: String
    )
}
