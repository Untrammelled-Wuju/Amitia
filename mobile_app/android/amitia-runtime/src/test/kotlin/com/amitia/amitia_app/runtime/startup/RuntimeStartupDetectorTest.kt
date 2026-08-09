package com.amitia.amitia_app.runtime.startup

import com.amitia.amitia_app.runtime.connection.BackendEndpointPolicy
import com.amitia.amitia_app.runtime.connection.embeddedAndroidBackendPolicy
import com.amitia.amitia_app.runtime.proot.ProotSession
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.concurrent.atomic.AtomicInteger

class RuntimeStartupDetectorTest {

    private fun createAlwaysAliveSession(): ProotSession = object : ProotSession {
        override val sessionId: String = "test-session-alive"
        override fun isAlive(): Boolean = true
        override fun awaitExit(timeoutMillis: Long): Int? = null
        override fun stop(graceMillis: Long): com.amitia.amitia_app.runtime.proot.ProotStopResult =
            com.amitia.amitia_app.runtime.proot.ProotStopResult.AlreadyStopped(sessionId, null)
        override fun close() {}
    }

    private fun createDeadSession(exitCode: Int = -1): ProotSession = object : ProotSession {
        override val sessionId: String = "test-session-dead"
        override fun isAlive(): Boolean = false
        override fun awaitExit(timeoutMillis: Long): Int? = exitCode
        override fun stop(graceMillis: Long): com.amitia.amitia_app.runtime.proot.ProotStopResult =
            com.amitia.amitia_app.runtime.proot.ProotStopResult.AlreadyStopped(sessionId, exitCode)
        override fun close() {}
    }

    private fun createFakeProbe(
        livenessResponses: List<RuntimeHealthProbeResult>,
        readinessResponses: List<RuntimeHealthProbeResult>
    ): RuntimeHealthProbe {
        val livenessIndex = AtomicInteger(0)
        val readinessIndex = AtomicInteger(0)
        return object : RuntimeHealthProbe {
            override fun checkLiveness(endpoint: BackendEndpointPolicy): RuntimeHealthProbeResult {
                val index = livenessIndex.getAndIncrement()
                return if (index < livenessResponses.size) livenessResponses[index] else livenessResponses.last()
            }
            override fun checkReadiness(endpoint: BackendEndpointPolicy): RuntimeHealthProbeResult {
                val index = readinessIndex.getAndIncrement()
                return if (index < readinessResponses.size) readinessResponses[index] else readinessResponses.last()
            }
        }
    }

    @Test
    fun normalStartup_returnsReady() {
        val endpoint = embeddedAndroidBackendPolicy()
        val session = createAlwaysAliveSession()
        val probe = createFakeProbe(
            livenessResponses = listOf(
                RuntimeHealthProbeResult.Failure(RuntimeHealthProbeError.ConnectionRefused),
                RuntimeHealthProbeResult.Success(200)
            ),
            readinessResponses = listOf(
                RuntimeHealthProbeResult.Success(503),
                RuntimeHealthProbeResult.Success(200)
            )
        )
        val detector = DefaultRuntimeStartupDetector(
            healthProbe = probe,
            totalTimeoutMs = 5000L,
            initialPollIntervalMs = 50L,
            maxPollIntervalMs = 100L
        )
        val request = RuntimeStartupRequest(
            generation = 1L,
            session = session,
            endpoint = endpoint
        )
        val result = detector.awaitStartup(request)
        assertTrue(result is RuntimeStartupResult.Ready)
        assertEquals(1L, result.generation)
    }

    @Test
    fun prootEarlyExit_returnsFailedWithProotExited() {
        val endpoint = embeddedAndroidBackendPolicy()
        val session = createDeadSession(exitCode = 1)
        val probe = createFakeProbe(
            livenessResponses = listOf(RuntimeHealthProbeResult.Success(200)),
            readinessResponses = listOf(RuntimeHealthProbeResult.Success(200))
        )
        val detector = DefaultRuntimeStartupDetector(
            healthProbe = probe,
            totalTimeoutMs = 5000L,
            initialPollIntervalMs = 50L,
            maxPollIntervalMs = 100L
        )
        val request = RuntimeStartupRequest(
            generation = 1L,
            session = session,
            endpoint = endpoint
        )
        val result = detector.awaitStartup(request)
        assertTrue(result is RuntimeStartupResult.Failed)
        val failed = result as RuntimeStartupResult.Failed
        assertTrue(failed.error is RuntimeStartupError.ProotExited)
        assertEquals(1, (failed.error as RuntimeStartupError.ProotExited).exitCode)
    }

    @Test
    fun livenessTimeout_returnsFailedWithTimeout() {
        val endpoint = embeddedAndroidBackendPolicy()
        val session = createAlwaysAliveSession()
        val probe = createFakeProbe(
            livenessResponses = listOf(RuntimeHealthProbeResult.Failure(RuntimeHealthProbeError.ConnectionRefused)),
            readinessResponses = listOf(RuntimeHealthProbeResult.Success(503))
        )
        val detector = DefaultRuntimeStartupDetector(
            healthProbe = probe,
            totalTimeoutMs = 500L,
            initialPollIntervalMs = 50L,
            maxPollIntervalMs = 100L
        )
        val request = RuntimeStartupRequest(
            generation = 1L,
            session = session,
            endpoint = endpoint
        )
        val result = detector.awaitStartup(request)
        assertTrue(result is RuntimeStartupResult.Failed)
        val failed = result as RuntimeStartupResult.Failed
        assertTrue(failed.error is RuntimeStartupError.Timeout)
    }

    @Test
    fun readiness503_continuesPolling() {
        val endpoint = embeddedAndroidBackendPolicy()
        val session = createAlwaysAliveSession()
        val probe = createFakeProbe(
            livenessResponses = listOf(RuntimeHealthProbeResult.Success(200)),
            readinessResponses = listOf(
                RuntimeHealthProbeResult.Success(503),
                RuntimeHealthProbeResult.Success(503),
                RuntimeHealthProbeResult.Success(200)
            )
        )
        val detector = DefaultRuntimeStartupDetector(
            healthProbe = probe,
            totalTimeoutMs = 5000L,
            initialPollIntervalMs = 50L,
            maxPollIntervalMs = 100L
        )
        val request = RuntimeStartupRequest(
            generation = 1L,
            session = session,
            endpoint = endpoint
        )
        val result = detector.awaitStartup(request)
        assertTrue(result is RuntimeStartupResult.Ready)
    }

    @Test
    fun health401_returnsFailedWithAuthFailed() {
        val endpoint = embeddedAndroidBackendPolicy()
        val session = createAlwaysAliveSession()
        val probe = createFakeProbe(
            livenessResponses = listOf(RuntimeHealthProbeResult.Success(401)),
            readinessResponses = emptyList()
        )
        val detector = DefaultRuntimeStartupDetector(
            healthProbe = probe,
            totalTimeoutMs = 5000L,
            initialPollIntervalMs = 50L,
            maxPollIntervalMs = 100L
        )
        val request = RuntimeStartupRequest(
            generation = 1L,
            session = session,
            endpoint = endpoint
        )
        val result = detector.awaitStartup(request)
        assertTrue(result is RuntimeStartupResult.Failed)
        val failed = result as RuntimeStartupResult.Failed
        assertTrue(failed.error is RuntimeStartupError.HealthAuthFailed)
    }

    @Test
    fun health404_returnsFailedWithEndpointMissing() {
        val endpoint = embeddedAndroidBackendPolicy()
        val session = createAlwaysAliveSession()
        val probe = createFakeProbe(
            livenessResponses = listOf(RuntimeHealthProbeResult.Success(404)),
            readinessResponses = emptyList()
        )
        val detector = DefaultRuntimeStartupDetector(
            healthProbe = probe,
            totalTimeoutMs = 5000L,
            initialPollIntervalMs = 50L,
            maxPollIntervalMs = 100L
        )
        val request = RuntimeStartupRequest(
            generation = 1L,
            session = session,
            endpoint = endpoint
        )
        val result = detector.awaitStartup(request)
        assertTrue(result is RuntimeStartupResult.Failed)
        val failed = result as RuntimeStartupResult.Failed
        assertTrue(failed.error is RuntimeStartupError.HealthEndpointMissing)
    }

    @Test
    fun cancel_returnsCancelled() {
        val endpoint = embeddedAndroidBackendPolicy()
        val session = createAlwaysAliveSession()
        val probe = createFakeProbe(
            livenessResponses = listOf(RuntimeHealthProbeResult.Failure(RuntimeHealthProbeError.ConnectionRefused)),
            readinessResponses = listOf(RuntimeHealthProbeResult.Success(503))
        )
        val detector = DefaultRuntimeStartupDetector(
            healthProbe = probe,
            totalTimeoutMs = 30000L,
            initialPollIntervalMs = 50L,
            maxPollIntervalMs = 100L
        )
        val request = RuntimeStartupRequest(
            generation = 1L,
            session = session,
            endpoint = endpoint
        )
        val thread = Thread {
            Thread.sleep(100)
            detector.cancel()
        }.apply { isDaemon = true; start() }
        val result = detector.awaitStartup(request)
        assertTrue(result is RuntimeStartupResult.Cancelled)
    }

    @Test
    fun health403_returnsFailedWithAuthFailed() {
        val endpoint = embeddedAndroidBackendPolicy()
        val session = createAlwaysAliveSession()
        val probe = createFakeProbe(
            livenessResponses = listOf(RuntimeHealthProbeResult.Success(403)),
            readinessResponses = emptyList()
        )
        val detector = DefaultRuntimeStartupDetector(
            healthProbe = probe,
            totalTimeoutMs = 5000L,
            initialPollIntervalMs = 50L,
            maxPollIntervalMs = 100L
        )
        val request = RuntimeStartupRequest(
            generation = 1L,
            session = session,
            endpoint = endpoint
        )
        val result = detector.awaitStartup(request)
        assertTrue(result is RuntimeStartupResult.Failed)
        val failed = result as RuntimeStartupResult.Failed
        assertTrue(failed.error is RuntimeStartupError.HealthAuthFailed)
    }

    @Test
    fun readiness404_returnsFailedWithEndpointMissing() {
        val endpoint = embeddedAndroidBackendPolicy()
        val session = createAlwaysAliveSession()
        val probe = createFakeProbe(
            livenessResponses = listOf(RuntimeHealthProbeResult.Success(200)),
            readinessResponses = listOf(RuntimeHealthProbeResult.Success(404))
        )
        val detector = DefaultRuntimeStartupDetector(
            healthProbe = probe,
            totalTimeoutMs = 5000L,
            initialPollIntervalMs = 50L,
            maxPollIntervalMs = 100L
        )
        val request = RuntimeStartupRequest(
            generation = 1L,
            session = session,
            endpoint = endpoint
        )
        val result = detector.awaitStartup(request)
        assertTrue(result is RuntimeStartupResult.Failed)
        val failed = result as RuntimeStartupResult.Failed
        assertTrue(failed.error is RuntimeStartupError.HealthEndpointMissing)
    }
}
