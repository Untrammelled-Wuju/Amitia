package com.amitia.platform.notification

import android.app.Activity
import android.app.Application
import android.content.Context
import com.amitia.core.database.dao.ProactiveDao
import com.amitia.core.database.entity.ProactiveMessageEntity
import com.amitia.core.logging.Logger
import com.amitia.core.model.ProactiveMessageDto
import com.amitia.core.network.endpoint.RuntimeEndpointProvider
import com.amitia.core.network.sse.SseClient
import com.amitia.core.network.sse.SseEvent
import com.amitia.core.repository.CharacterRepository
import com.amitia.core.repository.ChatRepository
import dagger.hilt.android.qualifiers.ApplicationContext
import java.util.concurrent.ConcurrentHashMap
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.flowOn
import kotlinx.coroutines.launch
import kotlinx.serialization.json.Json

@Singleton
class ProactiveMessageObserver @Inject constructor(
    @ApplicationContext private val context: Context,
    private val sseClient: SseClient,
    private val endpointProvider: RuntimeEndpointProvider,
    private val notificationManager: NotificationManagerImpl,
    private val chatRepository: ChatRepository,
    private val characterRepository: CharacterRepository,
    private val proactiveDao: ProactiveDao,
    private val json: Json,
    private val logger: Logger
) {

    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private var observeJob: Job? = null

    private val proactiveMessagesFlow = MutableStateFlow<List<ProactiveMessageDto>>(emptyList())
    val proactiveMessages: StateFlow<List<ProactiveMessageDto>> = proactiveMessagesFlow.asStateFlow()

    private val incomingFlow = MutableSharedFlow<ProactiveMessageDto>(extraBufferCapacity = 64)
    val incomingMessages: SharedFlow<ProactiveMessageDto> = incomingFlow.asSharedFlow()

    private val foregroundState = MutableStateFlow(false)
    val isInForeground: StateFlow<Boolean> = foregroundState.asStateFlow()

    private val seenIds = ConcurrentHashMap.newKeySet<String>()
    private var startedActivities = 0

    init {
        registerForegroundTracker()
    }

    fun observe(): Flow<ProactiveMessageDto> {
        val endpoint = endpointProvider.currentEndpoint.value
        val url = endpoint.baseUrl() + "/api/proactive-sse"
        val headers = buildMap {
            val token = endpoint.authHeader()
            if (!token.isNullOrBlank()) {
                put("Authorization", "Bearer $token")
            }
        }
        return sseClient.connectGet(url, headers)
            .let { flow ->
                kotlinx.coroutines.flow.flow {
                    flow.collect { event ->
                        if (event.event == SseEvent.EVENT_PROACTIVE_MESSAGE) {
                            val dto = parseProactive(event.data)
                            if (dto != null) emit(dto)
                        }
                    }
                }
            }
            .flowOn(Dispatchers.IO)
    }

    suspend fun start() {
        if (observeJob?.isActive == true) {
            logger.i(TAG, "observer already running")
            return
        }
        restoreUnnotified()
        observeJob = scope.launch {
            observe().collect { dto ->
                handleProactiveMessage(dto)
            }
        }
        logger.i(TAG, "observer started")
    }

    suspend fun stop() {
        observeJob?.cancel()
        observeJob = null
        logger.i(TAG, "observer stopped")
    }

    fun observeProactiveMessages(): StateFlow<List<ProactiveMessageDto>> = proactiveMessages

    private suspend fun handleProactiveMessage(dto: ProactiveMessageDto) {
        if (dto.id.startsWith("http_") || dto.id == "error") {
            logger.w(TAG, "proactive stream error: ${dto.id}")
            return
        }
        if (!seenIds.add(dto.id)) {
            logger.d(TAG, "proactive already seen: ${dto.id}")
            return
        }
        val entity = ProactiveMessageEntity(
            id = dto.id,
            characterId = dto.characterId,
            content = dto.content,
            createdAt = parseTimestamp(dto.createdAt),
            isRead = dto.isRead,
            isNotified = false
        )
        proactiveDao.upsert(entity)
        publishToUi(dto)
        val characterId = dto.characterId
        if (characterId != null && !foregroundState.value) {
            val characterName = resolveCharacterName(characterId)
            notificationManager.showProactiveMessage(
                characterId = characterId,
                characterName = characterName,
                content = dto.content,
                messageId = dto.id,
                conversationId = dto.conversationId
            )
        }
        proactiveDao.markNotified(dto.id)
    }

    private fun publishToUi(dto: ProactiveMessageDto) {
        val current = proactiveMessagesFlow.value.toMutableList()
        val existingIndex = current.indexOfFirst { it.id == dto.id }
        if (existingIndex >= 0) {
            current[existingIndex] = dto
        } else {
            current.add(0, dto)
        }
        proactiveMessagesFlow.value = current.take(MAX_UI_KEEP)
        scope.launch { incomingFlow.emit(dto) }
    }

    private suspend fun restoreUnnotified() {
        val pending = proactiveDao.listUnnotified()
        if (pending.isEmpty()) return
        logger.i(TAG, "restoring ${pending.size} unnotified proactive messages")
        pending.forEach { entity ->
            val dto = ProactiveMessageDto(
                id = entity.id,
                characterId = entity.characterId,
                content = entity.content,
                createdAt = entity.createdAt.toString(),
                isRead = entity.isRead
            )
            publishToUi(dto)
            seenIds.add(entity.id)
            val characterId = entity.characterId ?: return@forEach
            if (!foregroundState.value) {
                val characterName = resolveCharacterName(characterId)
                notificationManager.showProactiveMessage(
                    characterId = characterId,
                    characterName = characterName,
                    content = entity.content,
                    messageId = entity.id
                )
            }
            proactiveDao.markNotified(entity.id)
        }
    }

    private suspend fun resolveCharacterName(characterId: String): String {
        return runCatching { characterRepository.get(characterId).name }
            .onFailure { logger.w(TAG, "resolve character name failed: ${it.message}") }
            .getOrDefault(characterId)
    }

    private fun parseProactive(data: String): ProactiveMessageDto? {
        return runCatching {
            json.decodeFromString(ProactiveMessageDto.serializer(), data)
        }.onFailure { t ->
            logger.w(TAG, "parse proactive failed: ${t.message}")
        }.getOrNull()
    }

    private fun parseTimestamp(value: String?): Long {
        if (value.isNullOrBlank()) return System.currentTimeMillis()
        return runCatching {
            if (value.matches(Regex("\\d+"))) value.toLong()
            else java.time.Instant.parse(value).toEpochMilli()
        }.getOrDefault(System.currentTimeMillis())
    }

    private fun registerForegroundTracker() {
        val app = context.applicationContext as? Application ?: return
        app.registerActivityLifecycleCallbacks(object : Application.ActivityLifecycleCallbacks {
            override fun onActivityCreated(activity: Activity, savedInstanceState: android.os.Bundle?) {}
            override fun onActivityStarted(activity: Activity) {
                startedActivities++
                if (startedActivities == 1) {
                    foregroundState.value = true
                    logger.d(TAG, "app entered foreground")
                }
            }
            override fun onActivityResumed(activity: Activity) {}
            override fun onActivityPaused(activity: Activity) {}
            override fun onActivityStopped(activity: Activity) {
                startedActivities = (startedActivities - 1).coerceAtLeast(0)
                if (startedActivities == 0) {
                    foregroundState.value = false
                    logger.d(TAG, "app entered background")
                }
            }
            override fun onActivitySaveInstanceState(activity: Activity, outState: android.os.Bundle) {}
            override fun onActivityDestroyed(activity: Activity) {}
        })
    }

    companion object {
        private const val TAG = "ProactiveMessageObserver"
        private const val MAX_UI_KEEP = 50
    }
}
