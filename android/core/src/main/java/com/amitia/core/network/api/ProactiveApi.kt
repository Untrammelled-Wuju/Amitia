package com.amitia.core.network.api

import com.amitia.core.model.ProactiveListResponse
import com.amitia.core.model.ProactiveMarkReadRequest
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.Query

interface ProactiveApi {

    @GET("/api/proactive/messages")
    suspend fun listProactive(
        @Query("page") page: Int = 1,
        @Query("pageSize") pageSize: Int = 20,
        @Query("onlyUnread") onlyUnread: Boolean = false,
        @Query("characterId") characterId: String? = null
    ): ProactiveListResponse

    @POST("/api/proactive/messages/read")
    suspend fun markRead(@Body request: ProactiveMarkReadRequest)
}
