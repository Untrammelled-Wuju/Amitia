package com.amitia.core.repository

import com.amitia.core.model.ModelConfigDto
import com.amitia.core.model.ModelConfigUpdateRequest
import com.amitia.core.model.ModelDto
import com.amitia.core.network.api.ModelApi
import com.amitia.core.network.client.AmitiaApiClient
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class ModelRepository @Inject constructor(
    private val apiClient: AmitiaApiClient
) {

    private val api: ModelApi by lazy { apiClient.service(ModelApi::class.java) }

    suspend fun list(
        type: String? = null,
        provider: String? = null
    ): List<ModelDto> {
        return api.listModels(type, provider)
    }

    suspend fun get(id: String): ModelDto {
        return api.getModel(id)
    }

    suspend fun create(model: ModelDto): ModelDto {
        return api.createModel(model)
    }

    suspend fun update(id: String, model: ModelDto): ModelDto {
        return api.updateModel(id, model)
    }

    suspend fun delete(id: String) {
        api.deleteModel(id)
    }

    suspend fun getConfig(): ModelConfigDto {
        return api.getConfig()
    }

    suspend fun updateConfig(request: ModelConfigUpdateRequest): ModelConfigDto {
        return api.updateConfig(request)
    }
}
