package com.amitia.core.model

import kotlinx.serialization.Serializable

@Serializable
data class ChannelDto(
    val id: String,
    val name: String,
    val type: String,
    val status: String? = null,
    val enabled: Boolean = false,
    val boundAt: String? = null,
    val lastActiveAt: String? = null,
    val config: Map<String, String> = emptyMap()
)

@Serializable
data class ChannelStatusDto(
    val channels: List<ChannelDto> = emptyList(),
    val totalActive: Int = 0,
    val totalBound: Int = 0
)

@Serializable
data class ChannelBindRequest(
    val channelType: String,
    val config: Map<String, String>
)
