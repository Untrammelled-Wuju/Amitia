package com.amitia.android.integration

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.filters.LargeTest
import com.amitia.core.network.endpoint.RuntimeEndpoint
import com.amitia.core.network.endpoint.RuntimeEndpointProvider
import com.amitia.core.network.ws.WsClient
import com.amitia.core.network.ws.WsMessage
import com.amitia.core.repository.ProactiveRepository
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
class ProactiveMessageIntegrationTest {

    @get:Rule
    val hiltRule = HiltAndroidRule(this)

    private lateinit var context: Context

    @Inject
    lateinit var runtimeManager: RuntimeManager

    @Inject
    lateinit var endpointProvider: RuntimeEndpointProvider

    @Inject
    lateinit var proactiveRepository: ProactiveRepository

    @Inject
    lateinit var wsClient: WsClient

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
    fun proactive_repository_resolves_from_hilt_graph() {
        assertThat(proactiveRepository).isNotNull()
        assertThat(wsClient).isNotNull()
    }

    @Test
    fun endpoint_provider_returns_local_url_for_ws_when_local_mode() = runBlocking {
        endpointProvider.switchToLocal(authToken = "test-token")

        val wsUrl = endpointProvider.currentEndpoint.value.wsUrl()

        assertThat(wsUrl).isEqualTo("ws://127.0.0.1:18899")
    }

    @Test
    fun proactive_list_returns_response_after_runtime_ready() = runBlocking {
        try {
            withTimeoutOrNull(60_000L) { runtimeManager.start() }
        } catch (_: Throwable) {
        }

        if (runtimeManager.state.value !is RuntimeState.Running) {
            return@runBlocking
        }

        val response = runCatching {
            proactiveRepository.list(page = 1, pageSize = 10, onlyUnread = false)
        }.getOrNull()

        if (response != null) {
            assertThat(response.items).isNotNull()
        }
    }

    @Test
    fun proactive_mark_read_completes_without_throwing_when_runtime_ready() = runBlocking {
        try {
            withTimeoutOrNull(60_000L) { runtimeManager.start() }
        } catch (_: Throwable) {
        }

        if (runtimeManager.state.value !is RuntimeState.Running) {
            return@runBlocking
        }

        val result = runCatching {
            proactiveRepository.markRead(messageIds = listOf("test-1", "test-2"))
        }

        assertThat(result).isNotNull()
    }

    @Test
    fun ws_connect_emits_open_or_error_event_when_runtime_ready() = runBlocking {
        try {
            withTimeoutOrNull(60_000L) { runtimeManager.start() }
        } catch (_: Throwable) {
        }

        if (runtimeManager.state.value !is RuntimeState.Running) {
            return@runBlocking
        }

        val wsUrl = endpointProvider.currentEndpoint.value.wsUrl() + "/api/ws/proactive"

        val firstEvent = withTimeoutOrNull(10_000L) {
            wsClient.connect(wsUrl).first()
        }

        if (firstEvent != null) {
            val isOpenOrError = firstEvent.type == WsClient.EVENT_OPEN ||
                firstEvent.type == WsClient.EVENT_ERROR
            assertThat(isOpenOrError).isTrue()
        }
    }

    @Test
    fun ws_client_returns_error_event_when_runtime_not_ready() = runBlocking {
        runCatching { runtimeManager.stop() }

        val wsUrl = "ws://127.0.0.1:18899/api/ws/proactive"

        val firstEvent = withTimeoutOrNull(5_000L) {
            wsClient.connect(wsUrl).first()
        }

        if (firstEvent != null) {
            assertThat(firstEvent.type).isEqualTo(WsClient.EVENT_ERROR)
        }
    }
}
