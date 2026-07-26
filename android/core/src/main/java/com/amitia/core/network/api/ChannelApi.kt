package com.amitia.core.network.api

import com.amitia.core.model.ChannelBindRequest
import com.amitia.core.model.ChannelDto
import com.amitia.core.model.ChannelStatusDto
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.POST

interface ChannelApi {

    @GET("/api/channels")
    suspend fun listChannels(): List<ChannelDto>

    @GET("/api/channels/status")
    suspend fun getStatus(): ChannelStatusDto

    @POST("/api/channels/bind")
    suspend fun bind(@Body request: ChannelBindRequest): ChannelDto

    @POST("/api/channels/unbind")
    suspend fun unbind(@Body request: ChannelBindRequest): ChannelDto
}
