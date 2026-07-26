package com.amitia.core.repository

import app.cash.turbine.test
import com.amitia.core.model.ConversationDto
import com.amitia.core.model.ConversationListResponse
import com.amitia.core.model.MessageDto
import com.amitia.core.model.MessageListResponse
import com.amitia.core.model.SendStreamRequest
import com.amitia.core.network.api.ChatApi
import com.amitia.core.network.api.ConversationCreateRequest
import com.amitia.core.network.client.AmitiaApiClient
import com.amitia.core.network.client.AmitiaApiException
import com.amitia.core.network.endpoint.RuntimeEndpoint
import com.amitia.core.network.endpoint.RuntimeEndpointProvider
import com.amitia.core.network.sse.SseClient
import com.amitia.core.network.sse.SseEvent
import com.google.common.truth.Truth.assertThat
import io.mockk.coEvery
import io.mockk.every
import io.mockk.mockk
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.Json
import org.junit.Test

class ChatRepositoryTest {

    private val api: ChatApi = mockk(relaxed = true)
    private val apiClient: AmitiaApiClient = mockk(relaxed = true)
    private val sseClient: SseClient = mockk(relaxed = true)
    private val endpointProvider: RuntimeEndpointProvider = mockk(relaxed = true)
    private val json: Json = Json { ignoreUnknownKeys = true; encodeDefaults = true }

    private val repository: ChatRepository = ChatRepository(apiClient, sseClient, endpointProvider, json)

    private fun stubLocalEndpoint() {
        every { endpointProvider.currentEndpoint } returns kotlinx.coroutines.flow.MutableStateFlow(
            RuntimeEndpoint.Local(authToken = "tok")
        )
        every { apiClient.service(ChatApi::class.java) } returns api
    }

    @Test
    fun getHistory_returns_messages_from_api() = runTest {
        stubLocalEndpoint()
        val messages = listOf(
            MessageDto(id = "m1", conversationId = "c1", role = "user", content = "hello"),
            MessageDto(id = "m2", conversationId = "c1", role = "assistant", content = "hi")
        )
        coEvery { api.listMessages("c1", 1, 50) } returns MessageListResponse(items = messages, total = 2, page = 1, pageSize = 50)

        val result = repository.getHistory("c1")

        assertThat(result.items).hasSize(2)
        assertThat(result.items.map { it.id }).containsExactly("m1", "m2")
    }

    @Test
    fun listConversations_returns_conversations_from_api() = runTest {
        stubLocalEndpoint()
        val conversations = listOf(
            ConversationDto(id = "c1", characterId = "char-1"),
            ConversationDto(id = "c2", characterId = "char-2")
        )
        coEvery { api.listConversations(1, 20) } returns ConversationListResponse(items = conversations, total = 2)

        val result = repository.listConversations()

        assertThat(result.items).hasSize(2)
        assertThat(result.items.map { it.id }).containsExactly("c1", "c2")
    }

    @Test
    fun createConversation_passes_correct_request_body() = runTest {
        stubLocalEndpoint()
        val expected = ConversationDto(id = "c-new", characterId = "char-1")
        coEvery {
            api.createConversation(ConversationCreateRequest(title = null, characterId = "char-1", channel = "web"))
        } returns expected

        val result = repository.createConversation(title = null, characterId = "char-1", channel = "web")

        assertThat(result.id).isEqualTo("c-new")
    }

    @Test
    fun deleteConversation_invokes_api_delete() = runTest {
        stubLocalEndpoint()
        coEvery { api.deleteConversation("c1") } returns Unit

        repository.deleteConversation("c1")

        io.mockk.coVerify { api.deleteConversation("c1") }
    }

    @Test
    fun deleteMessage_invokes_api_delete() = runTest {
        stubLocalEndpoint()
        coEvery { api.deleteMessage("m1") } returns Unit

        repository.deleteMessage("m1")

        io.mockk.coVerify { api.deleteMessage("m1") }
    }

    @Test
    fun sendStream_emits_events_from_sseClient_in_order() = runTest {
        stubLocalEndpoint()
        val events = listOf(
            SseEvent(event = SseEvent.EVENT_MESSAGE_START, data = "{}"),
            SseEvent(event = SseEvent.EVENT_TOKEN, data = "{\"token\":\"hello\"}"),
            SseEvent(event = SseEvent.EVENT_TOKEN, data = "{\"token\":\" world\"}"),
            SseEvent(event = SseEvent.EVENT_TERMINAL, data = "{}")
        )
        every { sseClient.connect(any(), any(), any()) } returns flowOf(*events.toTypedArray())

        val request = SendStreamRequest(
            conversationId = "c1",
            characterId = "char-1",
            content = "hi",
            channel = "web"
        )

        repository.sendStream(request).test {
            assertThat(awaitItem().event).isEqualTo(SseEvent.EVENT_MESSAGE_START)
            assertThat(awaitItem().event).isEqualTo(SseEvent.EVENT_TOKEN)
            assertThat(awaitItem().event).isEqualTo(SseEvent.EVENT_TOKEN)
            assertThat(awaitItem().event).isEqualTo(SseEvent.EVENT_TERMINAL)
            awaitComplete()
        }
    }

    @Test
    fun sendStream_includes_authorization_header_when_token_present() = runTest {
        stubLocalEndpoint()
        every { sseClient.connect(any(), any(), any()) } returns flowOf()

        val request = SendStreamRequest(
            conversationId = "c1",
            characterId = "char-1",
            content = "hi",
            channel = "web"
        )
        repository.sendStream(request)

        io.mockk.verify {
            sseClient.connect(
                eq("http://127.0.0.1:18899/api/web-chat/send-stream"),
                any(),
                match { it["Authorization"] == "Bearer tok" }
            )
        }
    }

    @Test
    fun mapError_passes_AmitiaApiException_through() {
        val ex = AmitiaApiException.Timeout
        val mapped = repository.mapError(ex)

        assertThat(mapped).isSameInstanceAs(ex)
    }

    @Test
    fun mapError_wraps_unknown_throwable() {
        val raw = RuntimeException("boom")
        val mapped = repository.mapError(raw)

        assertThat(mapped).isInstanceOf(AmitiaApiException.Unknown::class.java)
        assertThat((mapped as AmitiaApiException.Unknown).raw).isSameInstanceAs(raw)
    }

    @Test
    fun retryMessage_uses_correct_url_and_emits_events() = runTest {
        stubLocalEndpoint()
        val events = listOf(
            SseEvent(event = SseEvent.EVENT_TOKEN, data = "{\"token\":\"retry\"}"),
            SseEvent(event = SseEvent.EVENT_TERMINAL, data = "{}")
        )
        every { sseClient.connect(any(), any(), any()) } returns flowOf(*events.toTypedArray())

        repository.retryMessage("m1").test {
            assertThat(awaitItem().event).isEqualTo(SseEvent.EVENT_TOKEN)
            assertThat(awaitItem().event).isEqualTo(SseEvent.EVENT_TERMINAL)
            awaitComplete()
        }

        io.mockk.verify {
            sseClient.connect(
                eq("http://127.0.0.1:18899/api/web-chat/messages/m1/retry"),
                eq("{}"),
                any()
            )
        }
    }
}
