package com.amitia.runtime.extension

import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.serialization.json.JsonObject
import kotlinx.serialization.json.buildJsonObject
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.put

@Singleton
class RemoteToolExecutor @Inject constructor(
    private val apiClient: ExtensionApiClient
) : ToolExecutor {

    override suspend fun execute(
        request: ToolInvocationRequest,
        tool: ToolDefinition
    ): Result<ToolInvocationResult> = runCatching {
        val start = System.currentTimeMillis()
        val input = buildJsonObject {
            request.arguments.forEach { (key, value) -> put(key, value) }
            put("toolId", request.toolId)
            put("extensionId", request.extensionId)
        }

        val result = try {
            apiClient.invokeAction(tool.toolId, ACTION_EXECUTE, input)
        } catch (actionError: ExtensionApiException) {
            try {
                apiClient.executeSkill(tool.toolId, input)
            } catch (skillError: ExtensionApiException) {
                apiClient.invokeDesktopContribution(tool.toolId, request.extensionId, input)
            }
        }

        val success = result["ok"]?.jsonPrimitive?.content?.toBooleanStrictOrNull()
            ?: result["success"]?.jsonPrimitive?.content?.toBooleanStrictOrNull()
            ?: (result["error"] == null)

        val error = result["error"]?.jsonPrimitive?.content
        val output = result["output"] ?: result["result"] ?: result

        ToolInvocationResult(
            success = success,
            output = output,
            error = error,
            durationMs = System.currentTimeMillis() - start
        )
    }

    companion object {
        private const val ACTION_EXECUTE = "execute"
    }
}
