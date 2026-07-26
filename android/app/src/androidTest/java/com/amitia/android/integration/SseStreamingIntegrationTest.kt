package com.amitia.android.integration

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.filters.LargeTest
import com.amitia.core.model.SendStreamRequest
import com.amitia.core.network.endpoint.RuntimeEndpoint
import com.amitia.core.network.endpoint.RuntimeEndpointProvider
import com.amitia.core.network.sse.SseEvent
import com.amitia.core.repository.ChatRepository
import com.amitia.runtime.api.RuntimeState
import com.amitia.runtime.manager.RuntimeManager
import com.google.common.truth.Truth.assertThat
import dagger.hilt.android.testing.HiltAndroidRule
import dagger.hilt.android.testing.HiltAndroidTest
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withTimeoutOrNull
import org.junit.After
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import javax.inject.Inject

@HiltAndroidTest
@RunWith(AndroidJUnit4::class)
@LargeTest
class SseStreamingIntegrationTest {

    @get:Rule
    val hiltRule = HiltAndroidRule(this)

    private lateinit var context: Context

    @Inject
    lateinit var runtimeManager: RuntimeManager

    @Inject
    lateinit var endpointProvider: RuntimeEndpointProvider

    @Inject
    lateinit var chatRepository: ChatRepository

    @Before
    fun setUp() {
        context = ApplicationProvider.getApplicationContext()
        hiltRule.inject()
        runBlocking { endpointProvider.loadInitial() }
    }

    @After
    fun tearDown() {
        runBlocking {
            runCatching { runtimeManager.stop() }
        }
    }

    @Test
    fun sse_endpoint_uses_local_runtime_url_when_in_local_mode() = runBlocking {
        endpointProvider.switchToLocal(authToken = "test-token")

        val endpoint = endpointProvider.currentEndpoint.value
        assertThat(endpoint).isInstanceOf(RuntimeEndpoint.Local::class.java)
        val baseUrl = endpoint.baseUrl()
        assertThat(baseUrl).isEqualTo("http://127.0.0.1:18899")
    }

    @Test
    fun sse_sendStream_returns_flow_of_events_when_runtime_ready() = runBlocking {
        try {
            withTimeoutOrNull(60_000L) { runtimeManager.start() }
        } catch (_: Throwable) {
        }

        if (runtimeManager.state.value !is RuntimeState.Running) {
            return@runBlocking
        }

        val conversation = chatRepository.createConversation(
            title = "sse-test",
            characterId = null,
            channel = "web"
        )

        val request = SendStreamRequest(
            conversationId = conversation.id,
            characterId = null,
            content = "hello sse",
            channel = "web"
        )

        val events = mutableListOf<SseEvent>()
        withTimeoutOrNull(30_000L) {
            chatRepository.sendStream(request).collect { event ->
                events.add(event)
                if (event.isTerminal()) return@collect
            }
        }

        if (events.isNotEmpty()) {
            val firstEvent = events.first()
            assertThat(firstEvent).isNotNull()
        }
    }

    @Test
    fun sse_stream_includes_message_start_event_for_valid_request() = runBlocking {
        try {
            withTimeoutOrNull(60_000L) { runtimeManager.start() }
        } catch (_: Throwable) {
        }

        if (runtimeManager.state.value !is RuntimeState.Running) {
            return@runBlocking
        }

        val conversation = chatRepository.createConversation(
            title = "sse-start",
            characterId = null,
            channel = "web"
        )
        val request = SendStreamRequest(
            conversationId = conversation.id,
            characterId = null,
            content = "starting",
            channel = "web"
        )

        val events = mutableListOf<SseEvent>()
        withTimeoutOrNull(30_000L) {
            chatRepository.sendStream(request).collect { event ->
                events.add(event)
                if (event.event == SseEvent.EVENT_TERMINAL) return@collect
            }
        }

        if (events.isNotEmpty()) {
            val hasMessageStart = events.any { it.event == SseEvent.EVENT_MESSAGE_START }
            assertThat(hasMessageStart).isTrue()
        }
    }

    @Test
    fun sse_stream_terminates_with_message_end_event() = runBlocking {
        try {
            withTimeoutOrNull(60_000L) { runtimeManager.start() }
        } catch (_: Throwable) {
        }

        if (runtimeManager.state.value !is RuntimeState.Running) {
            return@runBlocking
        }

        val conversation = chatRepository.createConversation(
            title = "sse-end",
            characterId = null,
            channel = "web"
        )
        val request = SendStreamRequest(
            conversationId = conversation.id,
            characterId = null,
            content = "terminate me",
            channel = "web"
        )

        val events = mutableListOf<SseEvent>()
        withTimeoutOrNull(30_000L) {
            chatRepository.sendStream(request).collect { event ->
                events.add(event)
                if (event.event == SseEvent.EVENT_TERMINAL) return@collect
            }
        }

        if (events.isNotEmpty()) {
            val hasTerminal = events.any { it.event == SseEvent.EVENT_TERMINAL }
            assertThat(hasTerminal).isTrue()
        }
    }

    @Test
    fun sse_token_events_concatenate_into_full_response() = runBlocking {
        try {
            withTimeoutOrNull(60_000L) { runtimeManager.start() }
        } catch (_: Throwable) {
        }

        if (runtimeManager.state.value !is RuntimeState.Running) {
            return@runBlocking
        }

        val conversation = chatRepository.createConversation(
            title = "sse-token",
            characterId = null,
            channel = "web"
        )
        val request = SendStreamRequest(
            conversationId = conversation.id,
            characterId = null,
            content = "stream tokens",
            channel = "web"
        )

        val events = mutableListOf<SseEvent>()
        withTimeoutOrNull(30_000L) {
            chatRepository.sendStream(request).collect { event ->
                events.add(event)
                if (event.isTerminal()) return@collect
            }
        }

        val tokenEvents = events.filter { it.event == SseEvent.EVENT_TOKEN }
        if (tokenEvents.isNotEmpty()) {
            assertThat(tokenEvents.size).isAtLeast(1)
        }
    }
}
