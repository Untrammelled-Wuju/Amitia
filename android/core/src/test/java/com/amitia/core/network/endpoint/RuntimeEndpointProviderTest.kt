package com.amitia.core.network.endpoint

import androidx.test.core.app.ApplicationProvider
import com.google.common.truth.Truth.assertThat
import kotlinx.coroutines.runBlocking
import org.junit.After
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.RobolectricTestRunner
import org.robolectric.annotation.Config

@RunWith(RobolectricTestRunner::class)
@Config(sdk = [34], manifest = Config.NONE)
class RuntimeEndpointProviderTest {

    private lateinit var provider: RuntimeEndpointProvider

    @Before
    fun setUp() {
        val context = ApplicationProvider.getApplicationContext<android.content.Context>()
        provider = RuntimeEndpointProvider(context)
        runBlocking { provider.loadInitial() }
    }

    @After
    fun tearDown() {
        runBlocking {
            provider.switchToLocal(authToken = null)
        }
    }

    @Test
    fun initial_endpoint_is_local_by_default() {
        val endpoint = provider.currentEndpoint.value

        assertThat(endpoint).isInstanceOf(RuntimeEndpoint.Local::class.java)
        val local = endpoint as RuntimeEndpoint.Local
        assertThat(local.host).isEqualTo("127.0.0.1")
        assertThat(local.port).isEqualTo(18899)
    }

    @Test
    fun switchToLocal_updates_state_and_persists_mode() {
        runBlocking {
            provider.switchToRemote(baseUrl = "https://remote.example.com", authToken = "tok")
            provider.switchToLocal(authToken = "abc")

            val endpoint = provider.currentEndpoint.value
            assertThat(endpoint).isInstanceOf(RuntimeEndpoint.Local::class.java)
            assertThat((endpoint as RuntimeEndpoint.Local).authToken).isEqualTo("abc")

            val mode = provider.getCurrentMode()
            assertThat(mode).isEqualTo(RuntimeEndpointProvider.RuntimeMode.LOCAL)
        }
    }

    @Test
    fun switchToRemote_updates_state_and_persists_remote_baseUrl() {
        runBlocking {
            provider.switchToRemote(baseUrl = "https://api.example.com/", authToken = "tok-123")

            val endpoint = provider.currentEndpoint.value
            assertThat(endpoint).isInstanceOf(RuntimeEndpoint.Remote::class.java)
            val remote = endpoint as RuntimeEndpoint.Remote
            assertThat(remote.baseUrl).isEqualTo("https://api.example.com")
            assertThat(remote.authToken).isEqualTo("tok-123")

            val mode = provider.getCurrentMode()
            assertThat(mode).isEqualTo(RuntimeEndpointProvider.RuntimeMode.REMOTE)
        }
    }

    @Test
    fun switchToRemote_with_null_token_clears_stored_token() {
        runBlocking {
            provider.switchToRemote(baseUrl = "https://api.example.com", authToken = "tok-1")
            provider.switchToRemote(baseUrl = "https://api.example.com", authToken = null)

            val stored = provider.getStoredAuthToken()
            assertThat(stored).isNull()
        }
    }

    @Test
    fun observeEndpoint_emits_latest_endpoint_value() {
        runBlocking {
            provider.switchToRemote(baseUrl = "https://observed.example.com", authToken = null)
            val endpoint = provider.currentEndpoint.value
            assertThat(endpoint).isInstanceOf(RuntimeEndpoint.Remote::class.java)
            val flowEndpoint = provider.observeEndpoint()
            assertThat(flowEndpoint).isNotNull()
        }
    }

    @Test
    fun loadInitial_restores_remote_mode_after_restart() {
        runBlocking {
            provider.switchToRemote(baseUrl = "https://persisted.example.com", authToken = "tok")

            val freshContext = ApplicationProvider.getApplicationContext<android.content.Context>()
            val freshProvider = RuntimeEndpointProvider(freshContext)
            freshProvider.loadInitial()

            val endpoint = freshProvider.currentEndpoint.value
            assertThat(endpoint).isInstanceOf(RuntimeEndpoint.Remote::class.java)
            assertThat((endpoint as RuntimeEndpoint.Remote).baseUrl).isEqualTo("https://persisted.example.com")
        }
    }
}
