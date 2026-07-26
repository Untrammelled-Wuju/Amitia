package com.amitia.feature.chat

import app.cash.turbine.test
import com.amitia.core.model.Attachment
import com.amitia.core.model.ConversationDto
import com.amitia.core.model.ConversationListResponse
import com.amitia.core.model.MessageDto
import com.amitia.core.model.MessageListResponse
import com.amitia.core.model.SendStreamRequest
import com.amitia.core.network.connection.SessionManager
import com.amitia.core.network.sse.SseEvent
import com.amitia.core.repository.CharacterRepository
import com.amitia.core.repository.ChatRepository
import com.google.common.truth.Truth.assertThat
import io.mockk.coEvery
import io.mockk.coVerify
import io.mockk.every
import io.mockk.mockk
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.resetMain
import kotlinx.coroutines.test.runTest
import kotlinx.coroutines.test.setMain
import org.junit.After
import org.junit.Before
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class ChatViewModelTest {

    private val chatRepository: ChatRepository = mockk(relaxed = true)
    private val characterRepository: CharacterRepository = mockk(relaxed = true)
    private val sessionManager: SessionManager = mockk(relaxed = true)
    private val chatDataStore: ChatDataStore = mockk(relaxed = true)
    private val testDispatcher = UnconfinedTestDispatcher()

    private lateinit var viewModel: ChatViewModel

    private val conversation = ConversationDto(id = "conv-1", characterId = "char-1")

    @Before
    fun setUp() {
        Dispatchers.setMain(testDispatcher)
        coEvery { chatRepository.listConversations(any(), any()) } returns ConversationListResponse(
            items = listOf(conversation),
            total = 1
        )
        coEvery { chatRepository.createConversation(any(), any(), any()) } returns conversation
        coEvery { chatRepository.getHistory(any(), any(), any()) } returns MessageListResponse(
            items = emptyList(),
            total = 0
        )
        coEvery { chatDataStore.loadDraft(any()) } returns ""
        every { chatRepository.sendStream(any()) } returns flowOf()
        viewModel = ChatViewModel(chatRepository, characterRepository, sessionManager, chatDataStore)
    }

    @After
    fun tearDown() {
        Dispatchers.resetMain()
    }

    @Test
    fun loadConversation_loads_existing_conversation_for_character() = runTest {
        viewModel.loadConversation("char-1")
        advanceUntilIdle()

        val state = viewModel.state.value
        assertThat(state.conversation).isNotNull()
        assertThat(state.conversation?.id).isEqualTo("conv-1")
        coVerify { chatRepository.listConversations(page = 1, pageSize = 50) }
    }

    @Test
    fun loadConversation_creates_new_conversation_when_none_exists() = runTest {
        coEvery { chatRepository.listConversations(any(), any()) } returns ConversationListResponse(
            items = emptyList(),
            total = 0
        )
        val newConv = ConversationDto(id = "conv-new", characterId = "char-new")
        coEvery { chatRepository.createConversation(any(), eq("char-new"), any()) } returns newConv

        viewModel.loadConversation("char-new")
        advanceUntilIdle()

        assertThat(viewModel.state.value.conversation?.id).isEqualTo("conv-new")
    }

    @Test
    fun loadConversation_loads_existing_draft_for_character() = runTest {
        coEvery { chatDataStore.loadDraft("char-1") } returns "draft text"

        viewModel.loadConversation("char-1")
        advanceUntilIdle()

        assertThat(viewModel.state.value.draft).isEqualTo("draft text")
    }

    @Test
    fun saveDraft_persists_to_dataStore_and_updates_state() = runTest {
        viewModel.loadConversation("char-1")
        advanceUntilIdle()

        viewModel.saveDraft("new draft text")
        advanceUntilIdle()

        coVerify { chatDataStore.saveDraft("char-1", "new draft text") }
        assertThat(viewModel.state.value.draft).isEqualTo("new draft text")
    }

    @Test
    fun saveDraft_ignored_when_no_character_loaded() = runTest {
        viewModel.saveDraft("ignored")
        advanceUntilIdle()

        coVerify(exactly = 0) { chatDataStore.saveDraft(any(), any()) }
    }

    @Test
    fun updateInput_updates_draft_state_only() = runTest {
        viewModel.updateInput("typing...")

        assertThat(viewModel.state.value.draft).isEqualTo("typing...")
        coVerify(exactly = 0) { chatDataStore.saveDraft(any(), any()) }
    }

    @Test
    fun saveDraft_clears_dataStore_when_text_blank() = runTest {
        viewModel.loadConversation("char-1")
        advanceUntilIdle()

        viewModel.saveDraft("")
        advanceUntilIdle()

        coVerify { chatDataStore.saveDraft("char-1", "") }
    }

    @Test
    fun loadConversation_does_not_reload_same_character_if_already_loaded() = runTest {
        viewModel.loadConversation("char-1")
        advanceUntilIdle()
        val firstConversation = viewModel.state.value.conversation

        viewModel.loadConversation("char-1")
        advanceUntilIdle()

        assertThat(viewModel.state.value.conversation).isSameInstanceAs(firstConversation)
    }

    @Test
    fun sendMessage_appends_user_message_to_state_and_clears_draft() = runTest {
        viewModel.loadConversation("char-1")
        advanceUntilIdle()
        every { chatRepository.sendStream(any()) } returns flowOf(
            SseEvent(event = SseEvent.EVENT_MESSAGE_START, data = "{}"),
            SseEvent(event = SseEvent.EVENT_TERMINAL, data = "{}")
        )

        viewModel.sendMessage("hello there")
        advanceUntilIdle()

        val state = viewModel.state.value
        assertThat(state.messages.any { it.role == "user" && it.content == "hello there" }).isTrue()
        assertThat(state.draft).isEqualTo("")
    }

    @Test
    fun sendMessage_ignored_when_text_blank_and_no_attachments() = runTest {
        viewModel.loadConversation("char-1")
        advanceUntilIdle()

        viewModel.sendMessage("")
        advanceUntilIdle()

        assertThat(viewModel.state.value.messages).isEmpty()
    }

    @Test
    fun handleSseEvent_TOKEN_appends_token_to_assistant_message() = runTest {
        viewModel.loadConversation("char-1")
        advanceUntilIdle()
        every { chatRepository.sendStream(any()) } returns flowOf(
            SseEvent(event = SseEvent.EVENT_MESSAGE_START, data = "{}"),
            SseEvent(event = SseEvent.EVENT_TOKEN, data = "{\"token\":\"Hello\"}"),
            SseEvent(event = SseEvent.EVENT_TOKEN, data = "{\"token\":\" world\"}"),
            SseEvent(event = SseEvent.EVENT_TERMINAL, data = "{}")
        )

        viewModel.sendMessage("hi")
        advanceUntilIdle()

        val assistantMessage = viewModel.state.value.messages.firstOrNull { it.role == "assistant" }
        assertThat(assistantMessage).isNotNull()
        assertThat(assistantMessage!!.content).isEqualTo("Hello world")
        assertThat(assistantMessage.status).isEqualTo("completed")
    }

    @Test
    fun handleSseEvent_VOICE_AUDIO_sets_audioUrl_on_assistant_message() = runTest {
        viewModel.loadConversation("char-1")
        advanceUntilIdle()
        every { chatRepository.sendStream(any()) } returns flowOf(
            SseEvent(event = SseEvent.EVENT_MESSAGE_START, data = "{}"),
            SseEvent(event = SseEvent.EVENT_VOICE_AUDIO, data = "{\"audioUrl\":\"https://example.com/audio.mp3\"}"),
            SseEvent(event = SseEvent.EVENT_TERMINAL, data = "{}")
        )

        viewModel.sendMessage("hi")
        advanceUntilIdle()

        val assistantMessage = viewModel.state.value.messages.firstOrNull { it.role == "assistant" }
        assertThat(assistantMessage?.audioUrl).isEqualTo("https://example.com/audio.mp3")
        assertThat(assistantMessage?.contentType).isEqualTo("audio")
        assertThat(viewModel.state.value.lastVoiceAudioUrl).isEqualTo("https://example.com/audio.mp3")
    }

    @Test
    fun consumeVoiceAudio_clears_lastVoiceAudioUrl() = runTest {
        viewModel.loadConversation("char-1")
        advanceUntilIdle()
        every { chatRepository.sendStream(any()) } returns flowOf(
            SseEvent(event = SseEvent.EVENT_MESSAGE_START, data = "{}"),
            SseEvent(event = SseEvent.EVENT_VOICE_AUDIO, data = "{\"audioUrl\":\"https://example.com/a.mp3\"}"),
            SseEvent(event = SseEvent.EVENT_TERMINAL, data = "{}")
        )
        viewModel.sendMessage("hi")
        advanceUntilIdle()

        viewModel.consumeVoiceAudio()

        assertThat(viewModel.state.value.lastVoiceAudioUrl).isNull()
    }

    @Test
    fun deleteMessage_removes_message_from_state() = runTest {
        viewModel.loadConversation("char-1")
        advanceUntilIdle()
        val message = MessageDto(id = "m1", conversationId = "conv-1", role = "user", content = "hi")
        coEvery { chatRepository.getHistory(any(), any(), any()) } returns MessageListResponse(
            items = listOf(message),
            total = 1
        )
        viewModel.loadConversation("char-1")
        advanceUntilIdle()
        coEvery { chatRepository.deleteMessage("m1") } returns Unit

        viewModel.deleteMessage("m1")
        advanceUntilIdle()

        assertThat(viewModel.state.value.messages.any { it.id == "m1" }).isFalse()
    }

    @Test
    fun consumeError_clears_error_state() = runTest {
        coEvery { chatRepository.listConversations(any(), any()) } throws RuntimeException("network error")

        viewModel.loadConversation("char-1")
        advanceUntilIdle()
        assertThat(viewModel.state.value.error).isNotNull()

        viewModel.consumeError()

        assertThat(viewModel.state.value.error).isNull()
    }

    @Test
    fun loadHistoryPage_merges_messages_deduplicating_by_id() = runTest {
        viewModel.loadConversation("char-1")
        advanceUntilIdle()
        val existing = MessageDto(id = "m1", conversationId = "conv-1", role = "user", content = "old")
        coEvery { chatRepository.getHistory("conv-1", 1, 50) } returns MessageListResponse(
            items = listOf(existing),
            total = 1
        )
        viewModel.loadHistoryPage(1)
        advanceUntilIdle()

        val stateMessages = viewModel.state.value.messages
        assertThat(stateMessages.count { it.id == "m1" }).isEqualTo(1)
    }

    @Test
    fun sendMessage_sets_sending_state_during_stream() = runTest {
        viewModel.loadConversation("char-1")
        advanceUntilIdle()
        every { chatRepository.sendStream(any()) } returns flowOf(
            SseEvent(event = SseEvent.EVENT_MESSAGE_START, data = "{}"),
            SseEvent(event = SseEvent.EVENT_TERMINAL, data = "{}")
        )

        viewModel.sendMessage("hello")
        advanceUntilIdle()

        assertThat(viewModel.state.value.sending).isFalse()
        assertThat(viewModel.state.value.generating).isFalse()
    }

    @Test
    fun retryMessage_creates_placeholder_assistant_message() = runTest {
        viewModel.loadConversation("char-1")
        advanceUntilIdle()
        every { chatRepository.retryMessage(any()) } returns flowOf(
            SseEvent(event = SseEvent.EVENT_MESSAGE_START, data = "{}"),
            SseEvent(event = SseEvent.EVENT_TOKEN, data = "{\"token\":\"retry\"}"),
            SseEvent(event = SseEvent.EVENT_TERMINAL, data = "{}")
        )

        viewModel.retryMessage("msg-1")
        advanceUntilIdle()

        val assistant = viewModel.state.value.messages.firstOrNull { it.role == "assistant" }
        assertThat(assistant).isNotNull()
        assertThat(assistant!!.content).isEqualTo("retry")
    }

    @Test
    fun sendMessage_with_image_attachments_sets_image_contentType() = runTest {
        viewModel.loadConversation("char-1")
        advanceUntilIdle()
        every { chatRepository.sendStream(any()) } returns flowOf(
            SseEvent(event = SseEvent.EVENT_MESSAGE_START, data = "{}"),
            SseEvent(event = SseEvent.EVENT_TERMINAL, data = "{}")
        )

        viewModel.sendMessage("look at this", imageUrls = listOf("https://example.com/img.jpg"))
        advanceUntilIdle()

        val userMessage = viewModel.state.value.messages.firstOrNull { it.role == "user" }
        assertThat(userMessage?.contentType).isEqualTo("image")
        assertThat(userMessage?.imageUrl).isEqualTo("https://example.com/img.jpg")
    }
}
