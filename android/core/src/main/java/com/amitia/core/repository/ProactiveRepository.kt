package com.amitia.core.repository

import com.amitia.core.model.ProactiveListResponse
import com.amitia.core.model.ProactiveMarkReadRequest
import com.amitia.core.network.api.ProactiveApi
import com.amitia.core.network.client.AmitiaApiClient
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class ProactiveRepository @Inject constructor(
    private val apiClient: AmitiaApiClient
) {

    private val api: ProactiveApi by lazy { apiClient.service(ProactiveApi::class.java) }

    suspend fun list(
        page: Int = 1,
        pageSize: Int = 20,
        onlyUnread: Boolean = false,
        characterId: String? = null
    ): ProactiveListResponse {
        return api.listProactive(page, pageSize, onlyUnread, characterId)
    }

    suspend fun markRead(messageIds: List<String>) {
        api.markRead(ProactiveMarkReadRequest(messageIds = messageIds))
    }
}
