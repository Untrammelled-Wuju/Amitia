package com.amitia.core.repository

import com.amitia.core.model.TtsRequest
import com.amitia.core.model.TtsResponse
import com.amitia.core.model.TtsVoiceDto
import java.util.UUID
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class TtsRepository @Inject constructor() {

    private val mockVoices = listOf(
        TtsVoiceDto(id = "voice_alloy", name = "Alloy", language = "zh-CN", gender = "neutral"),
        TtsVoiceDto(id = "voice_echo", name = "Echo", language = "zh-CN", gender = "male"),
        TtsVoiceDto(id = "voice_fable", name = "Fable", language = "zh-CN", gender = "female"),
        TtsVoiceDto(id = "voice_nova", name = "Nova", language = "zh-CN", gender = "female"),
        TtsVoiceDto(id = "voice_shimmer", name = "Shimmer", language = "zh-CN", gender = "female")
    )

    suspend fun synthesize(request: TtsRequest): TtsResponse {
        return TtsResponse(
            audioUrl = "https://mock.example.com/audio/${UUID.randomUUID()}.mp3",
            duration = 3.5,
            format = "mp3",
            size = 56000
        )
    }

    suspend fun getVoices(): List<TtsVoiceDto> {
        return mockVoices
    }
}
