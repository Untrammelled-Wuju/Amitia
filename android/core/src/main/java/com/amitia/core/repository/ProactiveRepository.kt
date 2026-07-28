package com.amitia.core.repository

import com.amitia.core.model.ProactiveListResponse
import com.amitia.core.model.ProactiveMessageDto
import javax.inject.Inject
import javax.inject.Singleton

@Singleton
class ProactiveRepository @Inject constructor() {

    suspend fun list(
        page: Int = 1,
        pageSize: Int = 20,
        onlyUnread: Boolean = false,
        characterId: String? = null
    ): ProactiveListResponse {
        return ProactiveListResponse(
            items = emptyList(),
            total = 0,
            page = page,
            pageSize = pageSize
        )
    }

    suspend fun markRead(messageIds: List<String>) {}
}
