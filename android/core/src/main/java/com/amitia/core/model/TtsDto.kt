package com.amitia.core.model

import kotlinx.serialization.Serializable

@Serializable
data class TtsRequest(
    val text: String,
    val voiceId: String? = null,
    val speed: Double? = null,
    val pitch: Double? = null,
    val volume: Double? = null,
    val format: String? = null,
    val characterId: String? = null
)

@Serializable
data class TtsResponse(
    val audioUrl: String,
    val duration: Double? = null,
    val format: String? = null,
    val size: Long? = null
)

@Serializable
data class TtsVoiceDto(
    val id: String,
    val name: String,
    val language: String? = null,
    val gender: String? = null,
    val previewUrl: String? = null
)
