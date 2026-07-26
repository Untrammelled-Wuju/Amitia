package com.amitia.feature.chat

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.amitia.core.model.Attachment
import com.amitia.core.model.ConversationDto
import com.amitia.core.model.MessageDto
import com.amitia.core.model.SendStreamRequest
import com.amitia.core.network.client.AmitiaApiException
import com.amitia.core.network.connection.SessionManager
import com.amitia.core.network.sse.SseEvent
import com.amitia.core.repository.CharacterRepository
import com.amitia.core.repository.ChatRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.UUID
import javax.inject.Inject
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch

@HiltViewModel
class ChatViewModel @Inject constructor(
    private val chatRepository: ChatRepository,
    private val characterRepository: CharacterRepository,
    private val sessionManager: SessionManager,
    private val chatDataStore: ChatDataStore
) : ViewModel() {

    private val _state = MutableStateFlow(ChatUiState())
    val state: StateFlow<ChatUiState> = _state.asStateFlow()

    private var streamJob: Job? = null
    private var currentCharacterId: String? = null
    private val pageSize = 50
    private val maxRetries = 3

    fun loadConversation(characterId: String) {
        if (characterId == currentCharacterId && _state.value.conversation != null) return
        currentCharacterId = characterId
        viewModelScope.launch {
            _state.value = _state.value.copy(loading = true, error = null, messages = emptyList())
            runCatching {
                val conversations = chatRepository.listConversations(page = 1, pageSize = 50)
                val existing = conversations.items.firstOrNull { it.characterId == characterId }
                val conversation = existing ?: chatRepository.createConversation(
                    title = null,
                    characterId = characterId,
                    channel = "web"
                )
                _state.value = _state.value.copy(conversation = conversation)
                loadHistoryPage(page = 1)
                val draft = chatDataStore.loadDraft(characterId)
                _state.value = _state.value.copy(draft = draft)
            }.onFailure { e ->
                _state.value = _state.value.copy(
                    loading = false,
                    error = e.message ?: "加载会话失败"
                )
            }
        }
    }

    fun loadHistoryPage(page: Int) {
        val conversation = _state.value.conversation ?: return
        viewModelScope.launch {
            runCatching { chatRepository.getHistory(conversation.id, page = page, pageSize = pageSize) }
                .onSuccess { response ->
                    val merged = (response.items + _state.value.messages).distinctBy { it.id }
                    _state.value = _state.value.copy(
                        messages = merged.sortedBy { it.createdAt ?: "" },
                        loading = false,
                        hasMore = response.items.size >= pageSize,
                        currentPage = page
                    )
                }
                .onFailure { e ->
                    _state.value = _state.value.copy(
                        loading = false,
                        error = e.message ?: "加载历史失败"
                    )
                }
        }
    }

    fun loadOlder() {
        val next = _state.value.currentPage + 1
        if (_state.value.hasMore) loadHistoryPage(page = next)
    }

    fun saveDraft(text: String) {
        val characterId = currentCharacterId ?: return
        viewModelScope.launch {
            chatDataStore.saveDraft(characterId, text)
            _state.value = _state.value.copy(draft = text)
        }
    }

    fun updateInput(text: String) {
        _state.value = _state.value.copy(draft = text)
    }

    fun sendMessage(text: String, imageUrls: List<String> = emptyList(), audioUrl: String? = null) {
        val conversation = _state.value.conversation
        val characterId = currentCharacterId ?: return
        if (text.isBlank() && imageUrls.isEmpty() && audioUrl.isNullOrBlank()) return

        val userMessage = MessageDto(
            id = UUID.randomUUID().toString(),
            conversationId = conversation?.id,
            role = "user",
            channel = "web",
            content = text,
            contentType = if (imageUrls.isNotEmpty()) "image" else if (audioUrl != null) "audio" else "text",
            status = "sent",
            imageUrl = imageUrls.firstOrNull(),
            audioUrl = audioUrl,
            createdAt = nowIso()
        )
        _state.value = _state.value.copy(
            messages = _state.value.messages + userMessage,
            draft = "",
            sending = true,
            error = null
        )
        viewModelScope.launch {
            chatDataStore.saveDraft(characterId, "")
        }
        startStream(text = text, characterId = characterId, conversationId = conversation?.id, attachments = buildAttachments(imageUrls, audioUrl))
    }

    fun retryMessage(messageId: String) {
        streamJob?.cancel()
        val placeholderId = UUID.randomUUID().toString()
        ensurePlaceholder(placeholderId)
        viewModelScope.launch {
            _state.value = _state.value.copy(generating = true, error = null)
            var attempt = 0
            var success = false
            while (attempt < maxRetries && !success) {
                attempt++
                runCatching {
                    chatRepository.retryMessage(messageId).collect { event ->
                        handleSseEvent(event, placeholderId)
                        if (event.event == SseEvent.EVENT_TERMINAL) success = true
                    }
                }.onFailure { e ->
                    if (attempt >= maxRetries) {
                        _state.value = _state.value.copy(
                            generating = false,
                            sending = false,
                            error = mapError(e)
                        )
                    }
                }
                if (!success && attempt < maxRetries) {
                    kotlinx.coroutines.delay(1000L * attempt)
                }
            }
        }
    }

    private fun startStream(
        text: String,
        characterId: String,
        conversationId: String?,
        attachments: List<Attachment>
    ) {
        streamJob?.cancel()
        val placeholderId = UUID.randomUUID().toString()
        ensurePlaceholder(placeholderId)
        val request = SendStreamRequest(
            conversationId = conversationId,
            characterId = characterId,
            content = text,
            channel = "web",
            useMemory = true,
            useTts = false,
            useVision = attachments.any { it.type == "image" },
            attachments = attachments
        )
        viewModelScope.launch {
            _state.value = _state.value.copy(generating = true, sending = false, error = null)
            var attempt = 0
            var success = false
            while (attempt < maxRetries && !success) {
                attempt++
                runCatching {
                    chatRepository.sendStream(request).collect { event ->
                        handleSseEvent(event, placeholderId)
                        if (event.event == SseEvent.EVENT_TERMINAL) success = true
                    }
                }.onFailure { e ->
                    if (attempt >= maxRetries) {
                        _state.value = _state.value.copy(
                            generating = false,
                            sending = false,
                            error = mapError(e)
                        )
                    }
                }
                if (!success && attempt < maxRetries) {
                    kotlinx.coroutines.delay(1000L * attempt)
                }
            }
        }
    }

    private fun ensurePlaceholder(placeholderId: String) {
        val placeholder = MessageDto(
            id = placeholderId,
            conversationId = _state.value.conversation?.id,
            role = "assistant",
            channel = "web",
            content = "",
            contentType = "text",
            status = "streaming",
            createdAt = nowIso()
        )
        _state.value = _state.value.copy(messages = _state.value.messages + placeholder)
    }

    private fun handleSseEvent(event: SseEvent, placeholderId: String) {
        when (event.event) {
            SseEvent.EVENT_MESSAGE_START -> {
                updateMessage(placeholderId) { current ->
                    current.copy(status = "streaming")
                }
            }
            SseEvent.EVENT_TOKEN -> {
                val token = extractTokenText(event.data)
                if (token.isNotEmpty()) {
                    updateMessage(placeholderId) { current ->
                        current.copy(content = current.content + token)
                    }
                }
            }
            SseEvent.EVENT_VOICE_AUDIO -> {
                val audio = extractField(event.data, "audioUrl") ?: extractField(event.data, "url")
                if (!audio.isNullOrBlank()) {
                    updateMessage(placeholderId) { current ->
                        current.copy(audioUrl = audio, contentType = "audio")
                    }
                    _state.value = _state.value.copy(lastVoiceAudioUrl = audio)
                }
            }
            SseEvent.EVENT_MESSAGE_CREATED, SseEvent.EVENT_MESSAGE_UPDATED -> {
                val realId = extractField(event.data, "id")
                if (!realId.isNullOrBlank()) {
                    updateMessage(placeholderId) { current -> current.copy(id = realId) }
                }
            }
            SseEvent.EVENT_TERMINAL -> {
                updateMessage(placeholderId) { current -> current.copy(status = "completed") }
                _state.value = _state.value.copy(generating = false, sending = false)
            }
            SseEvent.EVENT_ERROR -> {
                updateMessage(placeholderId) { current -> current.copy(status = "failed") }
                _state.value = _state.value.copy(
                    generating = false,
                    sending = false,
                    error = event.data.ifBlank { "流式回复失败" }
                )
            }
            else -> Unit
        }
    }

    private fun updateMessage(messageId: String, block: (MessageDto) -> MessageDto) {
        _state.value = _state.value.copy(
            messages = _state.value.messages.map { if (it.id == messageId) block(it) else it }
        )
    }

    fun deleteMessage(messageId: String) {
        viewModelScope.launch {
            runCatching { chatRepository.deleteMessage(messageId) }
                .onSuccess {
                    _state.value = _state.value.copy(
                        messages = _state.value.messages.filterNot { it.id == messageId }
                    )
                }
                .onFailure { e ->
                    _state.value = _state.value.copy(error = e.message ?: "删除消息失败")
                }
        }
    }

    fun copyMessage(messageId: String, onCopy: (String) -> Unit) {
        val message = _state.value.messages.firstOrNull { it.id == messageId } ?: return
        onCopy(message.content)
    }

    fun consumeError() {
        _state.value = _state.value.copy(error = null)
    }

    fun consumeVoiceAudio() {
        _state.value = _state.value.copy(lastVoiceAudioUrl = null)
    }

    private fun buildAttachments(imageUrls: List<String>, audioUrl: String?): List<Attachment> {
        val images = imageUrls.map { url ->
            Attachment(
                type = "image",
                url = url,
                mimeType = "image/jpeg",
                filename = "image.jpg"
            )
        }
        val audio = audioUrl?.let {
            listOf(
                Attachment(
                    type = "audio",
                    url = it,
                    mimeType = "audio/aac",
                    filename = "voice.aac"
                )
            )
        } ?: emptyList()
        return images + audio
    }

    private fun extractTokenText(data: String): String {
        if (data.isBlank()) return ""
        return runCatching {
            val json = kotlinx.serialization.json.Json { ignoreUnknownKeys = true }
            val obj = json.parseToJsonElement(data).let { it as? kotlinx.serialization.json.JsonObject }
                ?: return@runCatching data
            val tokenEl = obj["token"] ?: obj["text"]
            val primitive = tokenEl as? kotlinx.serialization.json.JsonPrimitive
            primitive?.content ?: data
        }.getOrDefault(data)
    }

    private fun extractField(data: String, field: String): String? {
        if (data.isBlank()) return null
        return runCatching {
            val json = kotlinx.serialization.json.Json { ignoreUnknownKeys = true }
            val obj = json.parseToJsonElement(data).let { it as? kotlinx.serialization.json.JsonObject }
                ?: return@runCatching null
            val el = obj[field] ?: return@runCatching null
            val primitive = el as? kotlinx.serialization.json.JsonPrimitive
            primitive?.content
        }.getOrNull()
    }

    private fun mapError(throwable: Throwable): String {
        return when (throwable) {
            is AmitiaApiException.SseDisconnected -> "流式连接断开：${throwable.message}"
            is AmitiaApiException -> throwable.message ?: "请求失败"
            else -> throwable.message ?: "请求失败"
        }
    }

    private fun nowIso(): String {
        val sdf = SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss.SSSXXX", Locale.getDefault())
        return sdf.format(Date())
    }

    override fun onCleared() {
        super.onCleared()
        streamJob?.cancel()
    }
}

data class ChatUiState(
    val conversation: ConversationDto? = null,
    val messages: List<MessageDto> = emptyList(),
    val draft: String = "",
    val sending: Boolean = false,
    val generating: Boolean = false,
    val loading: Boolean = false,
    val error: String? = null,
    val hasMore: Boolean = false,
    val currentPage: Int = 1,
    val lastVoiceAudioUrl: String? = null
)
