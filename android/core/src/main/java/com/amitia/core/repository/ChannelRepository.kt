package com.amitia.core.repository

import com.amitia.core.model.ChannelBindRequest
import com.amitia.core.model.ChannelDto
import com.amitia.core.model.ChannelStatusDto
import com.amitia.core.network.api.ChannelApi
import com.amitia.core.network.client.AmitiaApiClient
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class ChannelRepository @Inject constructor(
    private val apiClient: AmitiaApiClient
) {

    private val api: ChannelApi by lazy { apiClient.service(ChannelApi::class.java) }

    suspend fun list(): List<ChannelDto> {
        return api.listChannels()
    }

    suspend fun getStatus(): ChannelStatusDto {
        return api.getStatus()
    }

    suspend fun bind(channelType: String, config: Map<String, String>): ChannelDto {
        return api.bind(ChannelBindRequest(channelType = channelType, config = config))
    }

    suspend fun unbind(channelType: String, config: Map<String, String>): ChannelDto {
        return api.unbind(ChannelBindRequest(channelType = channelType, config = config))
    }
}
