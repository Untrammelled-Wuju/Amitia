package com.amitia.core.network.api

import com.amitia.core.model.TtsRequest
import com.amitia.core.model.TtsResponse
import com.amitia.core.model.TtsVoiceDto
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.POST

interface TtsApi {

    @POST("/api/tts/synthesize")
    suspend fun synthesize(@Body request: TtsRequest): TtsResponse

    @GET("/api/tts/voices")
    suspend fun getVoices(): List<TtsVoiceDto>
}
