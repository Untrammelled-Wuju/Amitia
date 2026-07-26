package com.amitia.core.network.api

import com.amitia.core.model.MemoryCreateRequest
import com.amitia.core.model.MemoryDto
import com.amitia.core.model.MemoryGraphDto
import com.amitia.core.model.MemorySearchRequest
import com.amitia.core.model.MemoryTimelineItem
import com.amitia.core.model.MemoryUpdateRequest
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.PUT
import retrofit2.http.Path
import retrofit2.http.Query

interface MemoryApi {

    @GET("/api/memory")
    suspend fun listMemories(
        @Query("page") page: Int = 1,
        @Query("pageSize") pageSize: Int = 20,
        @Query("characterId") characterId: String? = null,
        @Query("type") type: String? = null
    ): List<MemoryDto>

    @POST("/api/memory/search")
    suspend fun search(@Body request: MemorySearchRequest): List<MemoryDto>

    @GET("/api/memory/timeline")
    suspend fun getTimeline(
        @Query("start") start: String? = null,
        @Query("end") end: String? = null,
        @Query("limit") limit: Int = 50
    ): List<MemoryTimelineItem>

    @GET("/api/memory/graph")
    suspend fun getGraph(
        @Query("characterId") characterId: String? = null,
        @Query("depth") depth: Int = 2
    ): MemoryGraphDto

    @GET("/api/memory/{id}")
    suspend fun getMemory(@Path("id") id: String): MemoryDto

    @POST("/api/memory")
    suspend fun createMemory(@Body request: MemoryCreateRequest): MemoryDto

    @PUT("/api/memory/{id}")
    suspend fun updateMemory(
        @Path("id") id: String,
        @Body request: MemoryUpdateRequest
    ): MemoryDto

    @DELETE("/api/memory/{id}")
    suspend fun deleteMemory(@Path("id") id: String)
}
