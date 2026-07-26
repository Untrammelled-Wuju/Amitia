package com.amitia.core.network.api

import com.amitia.core.model.ConversationDto
import com.amitia.core.model.ConversationListResponse
import com.amitia.core.model.MessageDto
import com.amitia.core.model.MessageListResponse
import com.amitia.core.model.SendStreamRequest
import okhttp3.ResponseBody
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.Path
import retrofit2.http.Query

interface ChatApi {

    @POST("/api/web-chat/send-stream")
    suspend fun sendStream(@Body request: SendStreamRequest): ResponseBody

    @POST("/api/web-chat/send")
    suspend fun send(@Body request: SendStreamRequest): MessageDto

    @GET("/api/web-chat/conversations")
    suspend fun listConversations(
        @Query("page") page: Int = 1,
        @Query("pageSize") pageSize: Int = 20
    ): ConversationListResponse

    @POST("/api/web-chat/conversations")
    suspend fun createConversation(@Body request: ConversationCreateRequest): ConversationDto

    @GET("/api/web-chat/conversations/{id}/messages")
    suspend fun listMessages(
        @Path("id") conversationId: String,
        @Query("page") page: Int = 1,
        @Query("pageSize") pageSize: Int = 50
    ): MessageListResponse

    @DELETE("/api/web-chat/conversations/{id}")
    suspend fun deleteConversation(@Path("id") conversationId: String)

    @DELETE("/api/web-chat/messages/{id}")
    suspend fun deleteMessage(@Path("id") messageId: String)

    @POST("/api/web-chat/messages/{id}/retry")
    suspend fun retryMessage(@Path("id") messageId: String): ResponseBody
}

@kotlinx.serialization.Serializable
data class ConversationCreateRequest(
    val title: String? = null,
    val characterId: String? = null,
    val channel: String = "web"
)
