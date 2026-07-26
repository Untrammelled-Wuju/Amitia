package com.amitia.runtime.health

import com.google.common.truth.Truth.assertThat
import kotlinx.coroutines.delay
import kotlinx.coroutines.test.runTest
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Before
import org.junit.Test
import java.net.ServerSocket

class HealthCheckerImplTest {

    private lateinit var server: MockWebServer
    private lateinit var provider: OkHttpClientProvider
    private lateinit var checker: HealthCheckerImpl

    @Before
    fun setUp() {
        server = MockWebServer()
        server.start()
        provider = OkHttpClientProvider()
        checker = HealthCheckerImpl(provider)
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    @Test
    fun checkPort_returns_true_when_port_open() = runTest {
        val socket = ServerSocket(0)
        val port = socket.localPort
        try {
            val result = checker.checkPort("127.0.0.1", port, timeoutMs = 1000L)

            assertThat(result).isTrue()
        } finally {
            socket.close()
        }
    }

    @Test
    fun checkPort_returns_false_when_port_closed() = runTest {
        val socket = ServerSocket(0)
        val port = socket.localPort
        socket.close()

        val result = checker.checkPort("127.0.0.1", port, timeoutMs = 500L)

        assertThat(result).isFalse()
    }

    @Test
    fun checkPort_returns_false_for_unreachable_host() = runTest {
        val osName = System.getProperty("os.name") ?: ""
        val isWindows = osName.lowercase().contains("windows")
        org.junit.Assume.assumeFalse("Windows 防火墙可能拦截或代理不可达主机连接", isWindows)
        val result = checker.checkPort("192.0.2.1", 18899, timeoutMs = 200L)

        assertThat(result).isFalse()
    }

    @Test
    fun checkHttp_returns_true_on_200_status() = runTest {
        server.enqueue(MockResponse().setResponseCode(200).setBody("ok"))

        val url = server.url("/health").toString()
        val result = checker.checkHttp(url, timeoutMs = 2000L)

        assertThat(result).isTrue()
    }

    @Test
    fun checkHttp_returns_false_on_500_status() = runTest {
        server.enqueue(MockResponse().setResponseCode(500).setBody("error"))

        val url = server.url("/health").toString()
        val result = checker.checkHttp(url, timeoutMs = 2000L)

        assertThat(result).isFalse()
    }

    @Test
    fun checkHttp_returns_false_on_404_status() = runTest {
        server.enqueue(MockResponse().setResponseCode(404))

        val url = server.url("/health").toString()
        val result = checker.checkHttp(url, timeoutMs = 2000L)

        assertThat(result).isFalse()
    }

    @Test
    fun checkHttp_returns_false_when_connection_refused() = runTest {
        val socket = ServerSocket(0)
        val port = socket.localPort
        socket.close()

        val result = checker.checkHttp("http://127.0.0.1:$port/health", timeoutMs = 500L)

        assertThat(result).isFalse()
    }

    @Test
    fun checkProcess_returns_false_for_invalid_pid() = runTest {
        assertThat(checker.checkProcess(0)).isFalse()
        assertThat(checker.checkProcess(-1)).isFalse()
    }

    @Test
    fun waitForHealthy_returns_success_when_check_passes_first_time() = runTest {
        var calls = 0
        val result = checker.waitForHealthy(
            name = "test-service",
            check = { calls++; true },
            intervalMs = 50L,
            timeoutMs = 2000L
        )

        assertThat(result.isSuccess).isTrue()
        assertThat(calls).isEqualTo(1)
    }

    @Test
    fun waitForHealthy_returns_success_after_retries() = runTest {
        var calls = 0
        val result = checker.waitForHealthy(
            name = "test-service",
            check = {
                calls++
                calls >= 3
            },
            intervalMs = 50L,
            timeoutMs = 5000L
        )

        assertThat(result.isSuccess).isTrue()
        assertThat(calls).isAtLeast(3)
    }

    @Test
    fun waitForHealthy_returns_failure_on_timeout() = runTest {
        val result = checker.waitForHealthy(
            name = "slow-service",
            check = { false },
            intervalMs = 50L,
            timeoutMs = 200L
        )

        assertThat(result.isFailure).isTrue()
        val error = result.exceptionOrNull()
        assertThat(error).isInstanceOf(java.util.concurrent.TimeoutException::class.java)
        assertThat(error!!.message).contains("slow-service")
    }

    @Test
    fun waitForHealthy_swallows_exception_in_check_and_retries() = runTest {
        var calls = 0
        val result = checker.waitForHealthy(
            name = "flaky-service",
            check = {
                calls++
                if (calls < 2) throw IllegalStateException("transient")
                true
            },
            intervalMs = 50L,
            timeoutMs = 2000L
        )

        assertThat(result.isSuccess).isTrue()
        assertThat(calls).isAtLeast(2)
    }

    @Test
    fun buildServiceState_returns_Healthy_when_healthy_true() {
        val state = checker.buildServiceState(name = "qdrant", healthy = true, port = 18001)

        assertThat(state).isInstanceOf(com.amitia.runtime.api.ServiceState.Healthy::class.java)
        assertThat((state as com.amitia.runtime.api.ServiceState.Healthy).port).isEqualTo(18001)
    }

    @Test
    fun buildServiceState_returns_Unhealthy_when_reason_provided() {
        val state = checker.buildServiceState(name = "qdrant", healthy = false, reason = "timeout")

        assertThat(state).isInstanceOf(com.amitia.runtime.api.ServiceState.Unhealthy::class.java)
        assertThat((state as com.amitia.runtime.api.ServiceState.Unhealthy).reason).isEqualTo("timeout")
    }

    @Test
    fun buildServiceState_returns_Stopped_when_no_reason_no_port() {
        val state = checker.buildServiceState(name = "qdrant", healthy = false)

        assertThat(state).isEqualTo(com.amitia.runtime.api.ServiceState.Stopped)
    }
}
