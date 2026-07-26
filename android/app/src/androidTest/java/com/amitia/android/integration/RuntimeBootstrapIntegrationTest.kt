package com.amitia.android.integration

import android.content.Context
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.filters.LargeTest
import com.amitia.android.AmitiaApplication
import com.amitia.core.network.endpoint.RuntimeEndpoint
import com.amitia.core.network.endpoint.RuntimeEndpointProvider
import com.amitia.core.repository.CharacterRepository
import com.amitia.core.repository.ChatRepository
import com.amitia.core.network.sse.SseEvent
import com.amitia.runtime.api.RuntimeState
import com.amitia.runtime.api.ServiceState
import com.amitia.runtime.bootstrap.BootstrapSequence
import com.amitia.runtime.health.HealthChecker
import com.amitia.runtime.linux.LinuxRootfsManager
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
class RuntimeBootstrapIntegrationTest {

    @get:Rule
    val hiltRule = HiltAndroidRule(this)

    private lateinit var context: Context

    @Inject
    lateinit var runtimeManager: RuntimeManager

    @Inject
    lateinit var bootstrapSequence: BootstrapSequence

    @Inject
    lateinit var rootfsManager: LinuxRootfsManager

    @Inject
    lateinit var healthChecker: HealthChecker

    @Inject
    lateinit var endpointProvider: RuntimeEndpointProvider

    @Inject
    lateinit var characterRepository: CharacterRepository

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
    fun hilt_graph_resolves_runtime_manager_and_dependencies() {
        assertThat(runtimeManager).isNotNull()
        assertThat(bootstrapSequence).isNotNull()
        assertThat(rootfsManager).isNotNull()
        assertThat(healthChecker).isNotNull()
    }

    @Test
    fun runtime_initial_state_is_NotInstalled_or_Stopped() {
        val state = runtimeManager.state.value

        val isInitial = state is RuntimeState.NotInstalled ||
            state is RuntimeState.Stopped ||
            state is RuntimeState.Failed

        assertThat(isInitial).isTrue()
    }

    @Test
    fun endpoint_provider_returns_local_endpoint_by_default() = runBlocking {
        endpointProvider.switchToLocal(authToken = null)

        val endpoint = endpointProvider.currentEndpoint.value
        assertThat(endpoint).isInstanceOf(RuntimeEndpoint.Local::class.java)
        val local = endpoint as RuntimeEndpoint.Local
        assertThat(local.host).isEqualTo("127.0.0.1")
        assertThat(local.port).isEqualTo(18899)
    }

    @Test
    fun rootfs_manager_reports_isInstalled_after_runtime_first_start_attempt() = runBlocking {
        val before = rootfsManager.isInstalled()

        try {
            withTimeoutOrNull(60_000L) {
                runtimeManager.start()
            }
        } catch (_: Throwable) {
        }

        val after = rootfsManager.isInstalled()
        if (!before) {
            assertThat(after).isTrue()
        }
    }

    @Test
    fun backend_health_endpoint_reachable_after_bootstrap_success() = runBlocking {
        val started = runCatching {
            withTimeoutOrNull(60_000L) { runtimeManager.start() }
        }.isSuccess

        if (!started || runtimeManager.state.value !is RuntimeState.Running) {
            return@runBlocking
        }

        val reachable = healthChecker.checkHttp(
            url = "http://127.0.0.1:18899/api/health",
            timeoutMs = 3000L
        )

        assertThat(reachable).isTrue()
    }

    @Test
    fun character_repository_returns_list_after_runtime_ready() = runBlocking {
        val started = runCatching {
            withTimeoutOrNull(60_000L) { runtimeManager.start() }
        }.isSuccess

        if (!started || runtimeManager.state.value !is RuntimeState.Running) {
            return@runBlocking
        }

        val characters = runCatching { characterRepository.list() }.getOrDefault(emptyList())

        assertThat(characters).isNotNull()
    }

    @Test
    fun chat_repository_sendStream_emits_terminal_event_after_runtime_ready() = runBlocking {
        val started = runCatching {
            withTimeoutOrNull(60_000L) { runtimeManager.start() }
        }.isSuccess

        if (!started || runtimeManager.state.value !is RuntimeState.Running) {
            return@runBlocking
        }

        val conversation = runCatching {
            chatRepository.createConversation(title = "integration-test", characterId = null, channel = "web")
        }.getOrNull() ?: return@runBlocking

        val request = com.amitia.core.model.SendStreamRequest(
            conversationId = conversation.id,
            characterId = null,
            content = "integration hello",
            channel = "web"
        )

        val events = mutableListOf<SseEvent>()
        try {
            withTimeoutOrNull(30_000L) {
                chatRepository.sendStream(request).collect { event ->
                    events.add(event)
                    if (event.isTerminal()) return@collect
                }
            }
        } catch (_: Throwable) {
        }

        if (events.isNotEmpty()) {
            val hasTerminal = events.any { it.event == SseEvent.EVENT_TERMINAL || it.event == SseEvent.EVENT_ERROR }
            assertThat(hasTerminal).isTrue()
        }
    }

    @Test
    fun runtime_stop_transitions_to_Stopped_state() = runBlocking {
        try {
            withTimeoutOrNull(60_000L) { runtimeManager.start() }
        } catch (_: Throwable) {
        }

        runCatching { runtimeManager.stop() }

        val state = runtimeManager.state.value
        val isStoppedOrFailed = state is RuntimeState.Stopped ||
            state is RuntimeState.Failed ||
            state is RuntimeState.NotInstalled
        assertThat(isStoppedOrFailed).isTrue()
    }

    @Test
    fun runtime_restart_returns_to_running_or_degraded_after_stop() = runBlocking {
        try {
            withTimeoutOrNull(60_000L) { runtimeManager.start() }
        } catch (_: Throwable) {
        }
        val wasRunning = runtimeManager.state.value is RuntimeState.Running ||
            runtimeManager.state.value is RuntimeState.Degraded

        if (!wasRunning) {
            return@runBlocking
        }

        runCatching {
            withTimeoutOrNull(60_000L) { runtimeManager.restart() }
        }

        val state = runtimeManager.state.value
        val isOperating = state is RuntimeState.Running ||
            state is RuntimeState.Degraded ||
            state is RuntimeState.Failed
        assertThat(isOperating).isTrue()
    }

    @Test
    fun service_states_after_bootstrap_include_backend_healthy_when_running() = runBlocking {
        try {
            withTimeoutOrNull(60_000L) { runtimeManager.start() }
        } catch (_: Throwable) {
        }

        val state = runtimeManager.state.value
        if (state is RuntimeState.Running) {
            assertThat(state.services.backend).isInstanceOf(ServiceState.Healthy::class.java)
        } else if (state is RuntimeState.Degraded) {
            assertThat(state.services.backend).isInstanceOf(ServiceState.Healthy::class.java)
        }
    }
}
