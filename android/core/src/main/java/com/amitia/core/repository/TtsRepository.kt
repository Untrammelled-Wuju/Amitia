package com.amitia.core.repository

import com.amitia.core.model.TtsRequest
import com.amitia.core.model.TtsResponse
import com.amitia.core.model.TtsVoiceDto
import com.amitia.core.network.api.TtsApi
import com.amitia.core.network.client.AmitiaApiClient
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class TtsRepository @Inject constructor(
    private val apiClient: AmitiaApiClient
) {

    private val api: TtsApi by lazy { apiClient.service(TtsApi::class.java) }

    suspend fun synthesize(request: TtsRequest): TtsResponse {
        return api.synthesize(request)
    }

    suspend fun getVoices(): List<TtsVoiceDto> {
        return api.getVoices()
    }
}
