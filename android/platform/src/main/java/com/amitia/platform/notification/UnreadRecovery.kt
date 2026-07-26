package com.amitia.platform.notification

import com.amitia.core.logging.Logger
import com.amitia.core.repository.CharacterRepository
import com.amitia.core.repository.ProactiveRepository
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch

@Singleton
class UnreadRecovery @Inject constructor(
    private val proactiveRepository: ProactiveRepository,
    private val notificationManager: NotificationManagerImpl,
    private val characterRepository: CharacterRepository,
    private val logger: Logger
) {

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)

    fun recoverOnStartup() {
        scope.launch {
            runCatching {
                val response = proactiveRepository.list(page = 1, pageSize = 50, onlyUnread = true)
                if (response.items.isEmpty()) return@launch
                logger.i(TAG, "recovering ${response.items.size} unread proactive messages")
                response.items.forEach { dto ->
                    val characterId = dto.characterId ?: return@forEach
                    if (notificationManager.isNotified(dto.id)) return@forEach
                    val characterName = runCatching { characterRepository.get(characterId).name }
                        .getOrDefault(characterId)
                    notificationManager.showProactiveMessage(
                        characterId = characterId,
                        characterName = characterName,
                        content = dto.content,
                        messageId = dto.id,
                        conversationId = dto.conversationId
                    )
                }
            }.onFailure { t ->
                logger.w(TAG, "recoverOnStartup failed: ${t.message}")
            }
        }
    }

    companion object {
        private const val TAG = "UnreadRecovery"
    }
}
