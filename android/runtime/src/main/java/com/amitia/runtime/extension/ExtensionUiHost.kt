package com.amitia.runtime.extension

import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.contentOrNull
import kotlinx.serialization.json.jsonPrimitive

@Singleton
class ExtensionUiHost @Inject constructor(
    private val apiClient: ExtensionApiClient
) {

    suspend fun createWebUISession(
        contributionId: String,
        extensionId: String,
        moduleId: String,
        characterId: String? = null
    ): Result<WebUISession> = runCatching {
        val response = apiClient.createWebUISession(
            contributionId = contributionId,
            extensionId = extensionId,
            moduleId = moduleId,
            characterId = characterId
        )

        val sessionId = response["sessionId"]?.jsonPrimitive?.contentOrNull
            ?: throw IllegalStateException("Missing sessionId in WebUI session response")

        val entryUrl = response["entryUrl"]?.jsonPrimitive?.contentOrNull
            ?: response["resourceUrl"]?.jsonPrimitive?.contentOrNull
            ?: ""

        WebUISession(
            sessionId = sessionId,
            entryUrl = entryUrl,
            token = response["token"]?.jsonPrimitive?.contentOrNull,
            origin = response["origin"]?.jsonPrimitive?.contentOrNull,
            csp = response["csp"]?.jsonPrimitive?.contentOrNull
        )
    }

    fun buildWebUIUrl(sessionId: String, resourcePath: String = "index.html"): String {
        return apiClient.buildWebUIResourceUrl(sessionId, resourcePath)
    }

    suspend fun createSchemaSession(
        contributionId: String,
        extensionId: String,
        moduleId: String
    ): Result<WebUISession> = createWebUISession(contributionId, extensionId, moduleId)
}

data class WebUISession(
    val sessionId: String,
    val entryUrl: String,
    val token: String?,
    val origin: String?,
    val csp: String?
)
