package com.amitia.runtime.extension

import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.boolean
import kotlinx.serialization.json.booleanOrNull
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive

@Singleton
class ExtensionPermissionChecker @Inject constructor(
    private val apiClient: ExtensionApiClient
) {

    suspend fun checkPermission(extensionId: String, permission: String): Boolean {
        return runCatching {
            val response = apiClient.getPluginPermissions(extensionId)
            val permissions = response["permissions"]?.jsonArray
                ?: response["items"]?.jsonArray
                ?: return@runCatching false

            permissions.any { item ->
                val obj = item.jsonObject
                val permId = obj["id"]?.jsonPrimitive?.contentOrNull
                    ?: obj["name"]?.jsonPrimitive?.contentOrNull
                    ?: obj["permission"]?.jsonPrimitive?.contentOrNull
                val granted = obj["granted"]?.jsonPrimitive?.booleanOrNull
                    ?: obj["enabled"]?.jsonPrimitive?.booleanOrNull
                    ?: true
                permId == permission && granted
            }
        }.getOrDefault(false)
    }

    suspend fun revokePermission(extensionId: String, permission: String) {
        apiClient.updatePluginPermissions(extensionId, listOf(permission))
    }

    suspend fun listPermissions(extensionId: String): List<PermissionState> {
        return runCatching {
            val response = apiClient.getPluginPermissions(extensionId)
            val permissions = response["permissions"]?.jsonArray
                ?: response["items"]?.jsonArray
                ?: return@runCatching emptyList()

            permissions.mapNotNull { item ->
                val obj = item.jsonObject
                val permId = obj["id"]?.jsonPrimitive?.contentOrNull
                    ?: obj["name"]?.jsonPrimitive?.contentOrNull
                    ?: return@mapNotNull null
                val granted = obj["granted"]?.jsonPrimitive?.booleanOrNull
                    ?: obj["enabled"]?.jsonPrimitive?.booleanOrNull
                    ?: true
                PermissionState(
                    permissionId = permId,
                    granted = granted,
                    description = obj["description"]?.jsonPrimitive?.contentOrNull
                )
            }
        }.getOrDefault(emptyList())
    }
}

data class PermissionState(
    val permissionId: String,
    val granted: Boolean,
    val description: String?
)
