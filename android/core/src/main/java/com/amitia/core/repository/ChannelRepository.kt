package com.amitia.core.repository

import com.amitia.core.model.ChannelDto
import com.amitia.core.model.ChannelStatusDto
import java.util.UUID
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class ChannelRepository @Inject constructor() {

    private val mockChannels = listOf(
        ChannelDto(
            id = "ch_web",
            name = "Web 对话",
            type = "web",
            status = "active",
            enabled = true,
            boundAt = "2026-07-01T08:00:00",
            lastActiveAt = "2026-07-28T10:00:00"
        ),
        ChannelDto(
            id = "ch_wechat",
            name = "微信",
            type = "wechat",
            status = "inactive",
            enabled = false
        ),
        ChannelDto(
            id = "ch_qq",
            name = "QQ",
            type = "qq",
            status = "inactive",
            enabled = false
        )
    )

    suspend fun list(): List<ChannelDto> {
        return mockChannels
    }

    suspend fun getStatus(): ChannelStatusDto {
        return ChannelStatusDto(
            channels = mockChannels,
            totalActive = 1,
            totalBound = 1
        )
    }

    suspend fun bind(channelType: String, config: Map<String, String>): ChannelDto {
        return ChannelDto(
            id = UUID.randomUUID().toString(),
            name = channelType,
            type = channelType,
            status = "active",
            enabled = true,
            boundAt = java.text.SimpleDateFormat(
                "yyyy-MM-dd'T'HH:mm:ss.SSSXXX",
                java.util.Locale.getDefault()
            ).format(java.util.Date())
        )
    }

    suspend fun unbind(channelType: String, config: Map<String, String>): ChannelDto {
        return ChannelDto(
            id = "ch_unbind",
            name = channelType,
            type = channelType,
            status = "inactive",
            enabled = false
        )
    }
}
