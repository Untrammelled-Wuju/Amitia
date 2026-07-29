package com.amitia.runtime.extension

import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.serialization.json.Json
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonElement
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.add
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.put
import kotlinx.serialization.json.putJsonArray
import kotlinx.serialization.json.putJsonObject
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.MultipartBody
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody
import okhttp3.RequestBody.Companion.toRequestBody
import okhttp3.Response

@Singleton
class ExtensionApiClient @Inject constructor(
    private val httpClient: OkHttpClient,
    private val baseUrlProvider: BaseUrlProvider,
    private val json: Json
) {

    private val jsonMediaType = "application/json".toMediaType()

    suspend fun installExtension(packageBytes: ByteArray, fileName: String = "package.amitiax"): JsonObject {
        val requestBody = MultipartBody.Builder()
            .setType(MultipartBody.FORM)
            .addFormDataPart("package", fileName, packageBytes.toRequestBody())
            .build()
        val request = buildRequest("/api/extensions/kernel/extensions/install")
            .post(requestBody)
            .build()
        return executeRequest(request)
    }

    suspend fun enableExtension(extensionId: String): JsonObject {
        val body = buildJsonObject { put("id", extensionId) }
            .toString()
            .toRequestBody(jsonMediaType)
        val request = buildRequest("/api/extensions/kernel/extensions/enable?id=$extensionId")
            .post(body)
            .build()
        return executeRequest(request)
    }

    suspend fun disableExtension(extensionId: String): JsonObject {
        val body = buildJsonObject { put("id", extensionId) }
            .toString()
            .toRequestBody(jsonMediaType)
        val request = buildRequest("/api/extensions/kernel/extensions/disable?id=$extensionId")
            .post(body)
            .build()
        return executeRequest(request)
    }

    suspend fun uninstallExtension(extensionId: String): JsonObject {
        val body = buildJsonObject { put("id", extensionId) }
            .toString()
            .toRequestBody(jsonMediaType)
        val request = buildRequest("/api/extensions/kernel/extensions/uninstall?id=$extensionId")
            .post(body)
            .build()
        return executeRequest(request)
    }

    suspend fun listExtensions(): JsonObject {
        val request = buildRequest("/api/extensions/kernel/extensions")
            .get()
            .build()
        return executeRequest(request)
    }

    suspend fun getExtensionDetail(extensionId: String): JsonObject {
        val request = buildRequest("/api/extensions/kernel/extension?id=$extensionId")
            .get()
            .build()
        return executeRequest(request)
    }

    suspend fun executeSkill(skillId: String, input: JsonObject): JsonObject {
        val body = buildJsonObject {
            put("input", input)
        }.toString().toRequestBody(jsonMediaType)
        val request = buildRequest("/api/extensions/skills/$skillId/execute")
            .post(body)
            .build()
        return executeRequest(request)
    }

    suspend fun invokeAction(contributionId: String, actionId: String, input: JsonObject): JsonObject {
        val body = buildJsonObject {
            putJsonObject("context") {
                put("origin", "android")
            }
            put("input", input)
        }.toString().toRequestBody(jsonMediaType)
        val request = buildRequest("/api/extension/action/$contributionId/$actionId")
            .post(body)
            .build()
        return executeRequest(request)
    }

    suspend fun invokeDesktopContribution(contributionId: String, extensionId: String, input: JsonObject): JsonObject {
        val body = buildJsonObject {
            put("extensionId", extensionId)
            put("global", true)
            put("input", input)
        }.toString().toRequestBody(jsonMediaType)
        val request = buildRequest("/api/extensions/desktop/contributions/$contributionId/invoke")
            .post(body)
            .build()
        return executeRequest(request)
    }

    suspend fun listPlugins(): JsonObject {
        val request = buildRequest("/api/extensions/plugins")
            .get()
            .build()
        return executeRequest(request)
    }

    suspend fun getPluginPermissions(pluginId: String): JsonObject {
        val request = buildRequest("/api/extensions/plugins/$pluginId/permissions")
            .get()
            .build()
        return executeRequest(request)
    }

    suspend fun updatePluginPermissions(pluginId: String, revokedPermissions: List<String>): JsonObject {
        val body = buildJsonObject {
            putJsonArray("revoked") {
                revokedPermissions.forEach { add(it) }
            }
        }.toString().toRequestBody(jsonMediaType)
        val request = buildRequest("/api/extensions/plugins/$pluginId/permissions")
            .put(body)
            .build()
        return executeRequest(request)
    }

    suspend fun createWebUISession(
        contributionId: String,
        extensionId: String,
        moduleId: String,
        characterId: String? = null
    ): JsonObject {
        val body = buildJsonObject {
            put("contributionId", contributionId)
            put("extensionId", extensionId)
            put("moduleId", moduleId)
            put("surface", "android")
            if (characterId != null) put("characterId", characterId)
        }.toString().toRequestBody(jsonMediaType)
        val request = buildRequest("/api/extension/webui/session")
            .post(body)
            .build()
        return executeRequest(request)
    }

    fun buildWebUIResourceUrl(sessionId: String, resourcePath: String = "index.html"): String {
        return "${baseUrlProvider.baseUrl()}/api/extension/webui/resource/$sessionId/$resourcePath"
    }

    suspend fun listSchedules(): JsonObject {
        val request = buildRequest("/api/extensions/schedules")
            .get()
            .build()
        return executeRequest(request)
    }

    suspend fun runScheduleNow(scheduleId: String, source: String? = null): JsonObject {
        val builder = buildRequest("/api/extensions/schedules/$scheduleId/run-now")
        if (source != null) {
            builder.header("X-Schedule-Source", source)
        }
        val request = builder.post(EMPTY_BODY).build()
        return executeRequest(request)
    }

    suspend fun listTasks(): JsonObject {
        val request = buildRequest("/api/extensions/tasks")
            .get()
            .build()
        return executeRequest(request)
    }

    suspend fun enqueueTask(taskDefinitionId: String, extensionId: String, input: JsonObject): JsonObject {
        val body = buildJsonObject {
            put("taskDefinitionId", taskDefinitionId)
            put("extensionId", extensionId)
            put("input", input)
        }.toString().toRequestBody(jsonMediaType)
        val request = buildRequest("/api/extensions/tasks")
            .post(body)
            .build()
        return executeRequest(request)
    }

    private fun buildRequest(path: String): Request.Builder {
        val url = "${baseUrlProvider.baseUrl()}$path"
        return Request.Builder().url(url)
    }

    private fun executeRequest(request: Request): JsonObject {
        httpClient.newCall(request).execute().use { response ->
            val responseBody = response.body?.string().orEmpty()
            val parsed = if (responseBody.isNotBlank()) {
                json.parseToJsonElement(responseBody)
            } else {
                buildJsonObject { put("status", response.code) }
            }
            if (!response.isSuccessful) {
                val errorMessage = (parsed as? JsonObject)?.let {
                    it["error"]?.jsonPrimitive?.contentOrNull() ?: it["message"]?.jsonPrimitive?.contentOrNull()
                } ?: "HTTP ${response.code}"
                throw ExtensionApiException(response.code, errorMessage, parsed as? JsonObject)
            }
            return parsed as? JsonObject ?: buildJsonObject {
                put("status", "ok")
                put("raw", parsed.toString())
            }
        }
    }

    private fun JsonPrimitive.contentOrNull(): String? =
        if (this.isString) this.content else null

    companion object {
        private val EMPTY_BODY: RequestBody = "".toRequestBody("application/json".toMediaType())
    }
}

class ExtensionApiException(
    val statusCode: Int,
    val errorMessage: String,
    val response: JsonObject?
) : RuntimeException("Extension API error ($statusCode): $errorMessage")
