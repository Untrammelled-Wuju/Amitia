package com.amitia.amitia_app.runtime.startup

import com.amitia.amitia_app.runtime.connection.BackendEndpointPolicy
import com.amitia.amitia_app.runtime.connection.embeddedAndroidBackendPolicy
import com.amitia.amitia_app.runtime.internal.SystemRuntimeClock
import com.amitia.amitia_app.runtime.proot.ProotSession
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import java.util.concurrent.atomic.AtomicInteger
import java.util.concurrent.atomic.AtomicReference

class RuntimeStartupDetectorTest {

    private fun createAlwaysAliveSession(): ProotSession = object : ProotSession {
        override val sessionId: String = "test-session-alive"
        override fun isAlive(): Boolean = true
        override fun awaitExit(timeoutMillis: Long): Int? = null
        override fun stop(graceMillis: Long): com.amitia.amitia_app.runtime.proot.ProotStopResult =
            com.amitia.amitia_app.runtime.proot.ProotStopResult.AlreadyStopped(sessionId, null)
        override fun close() {}
    }

    private fun createLiveThenDeadSession(aliveForProbes: Int): ProotSession {
        val counter = AtomicInteger(0)
        return object : ProotSession {
            override val sessionId: String = "test-session-liven-die"
            override fun isAlive(): Boolean = counter.getAndIncrement() < aliveForProbes
            override fun awaitExit(timeoutMillis: Long): Int? = 137
            override fun stop(graceMillis: Long): com.amitia.amitia_app.runtime.proot.ProotStopResult =
                com.amitia.amitia_app.runtime.proot.ProotStopResult.AlreadyStopped(sessionId, 137)
            override fun close() {}
        }
    }

    private fun createDeadSession(exitCode: Int = -1): ProotSession = object : ProotSession {
        override val sessionId: String = "test-session-dead"
        override fun isAlive(): Boolean = false
        override fun awaitExit(timeoutMillis: Long): Int? = exitCode
        override fun stop(graceMillis: Long): com.amitia.amitia_app.runtime.proot.ProotStopResult =
            com.amitia.amitia_app.runtime.proot.ProotStopResult.AlreadyStopped(sessionId, exitCode)
        override fun close() {}
    }

    private fun createFakeReadinessProbe(responses: List<RuntimeHealthProbeResult>): RuntimeHealthProbe {
        val index = AtomicInteger(0)
        return object : RuntimeHealthProbe {
            override fun checkReadiness(endpoint: BackendEndpointPolicy): RuntimeHealthProbeResult {
                val i = index.getAndIncrement()
                return if (i < responses.size) responses[i] else responses.last()
            }
        }
    }

    private fun validReadyResponse(): String {
        return """{"code":200,"msg":"ready","data":{"status":"ready","state":"Ready","blockingCount":0,"degradedCount":0,"readyCount":1,"failedCount":0,"timestamp":"2026-01-01T00:00:00Z"}}"""
    }

    private fun validStartingResponse(): String {
        return """{"code":200,"msg":"starting","data":{"status":"starting","state":"Starting","blockingCount":0,"degradedCount":0,"readyCount":0,"failedCount":0,"timestamp":"2026-01-01T00:00:00Z"}}"""
    }

    private fun validDegradedResponse(): String {
        return """{"code":200,"msg":"degraded","data":{"status":"degraded","state":"Degraded","blockingCount":0,"degradedCount":1,"readyCount":0,"failedCount":0,"timestamp":"2026-01-01T00:00:00Z"}}"""
    }

    private fun blocked503Response(): String {
        return """{"code":503,"msg":"blocked","data":{"status":"blocked","state":"Blocked","blockingCount":1,"degradedCount":0,"readyCount":0,"failedCount":0,"timestamp":"2026-01-01T00:00:00Z"}}"""
    }

    private val validEndpoint = embeddedAndroidBackendPolicy()

    @Test
    fun validReadyResponse_returnsReady() {
        val session = createAlwaysAliveSession()
        val probe = createFakeReadinessProbe(
            listOf(
                RuntimeHealthProbeResult.Success(200, validReadyResponse())
            )
        )
        val detector = DefaultRuntimeStartupDetector(
            healthProbe = probe,
            totalStartupTimeoutMs = 5000L,
            initialProbeIntervalMs = 10L,
            maxProbeIntervalMs = 50L
        )
        val request = RuntimeStartupRequest(
            generation = 1L,
            session = session,
            endpoint = validEndpoint,
            startAttemptId = "attempt-1"
        )
        val result = detector.awaitStartup(request)
        assertTrue("Expected Ready but was $result", result is RuntimeStartupResult.Ready)
        assertEquals(1L, result.generation)
        val ready = result as RuntimeStartupResult.Ready
        assertTrue(ready.elapsedMs >= 0L)
        assertEquals(1, ready.probeCount)
    }

    @Test
    fun connectionRefusedInDeadline_returnsReadyEventually() {
        val session = createAlwaysAliveSession()
        val probe = createFakeReadinessProbe(
            listOf(
                RuntimeHealthProbeResult.Failure(RuntimeHealthProbeError.ConnectionRefused),
                RuntimeHealthProbeResult.Failure(RuntimeHealthProbeError.ConnectionRefused),
                RuntimeHealthProbeResult.Success(200, validReadyResponse())
            )
        )
        val detector = DefaultRuntimeStartupDetector(
            healthProbe = probe,
            totalStartupTimeoutMs = 5000L,
            initialProbeIntervalMs = 10L,
            maxProbeIntervalMs = 50L
        )
        val request = RuntimeStartupRequest(
            generation = 2L,
            session = session,
            endpoint = validEndpoint,
            startAttemptId = "attempt-2"
        )
        val result = detector.awaitStartup(request)
        assertTrue("Expected Ready but was $result", result is RuntimeStartupResult.Ready)
    }

    @Test
    fun readiness503_continuesPollingUntilReady() {
        val session = createAlwaysAliveSession()
        val probe = createFakeReadinessProbe(
            listOf(
                RuntimeHealthProbeResult.Success(503, blocked503Response()),
                RuntimeHealthProbeResult.Success(503, blocked503Response()),
                RuntimeHealthProbeResult.Success(200, validReadyResponse())
            )
        )
        val detector = DefaultRuntimeStartupDetector(
            healthProbe = probe,
            totalStartupTimeoutMs = 5000L,
            initialProbeIntervalMs = 10L,
            maxProbeIntervalMs = 50L
        )
        val request = RuntimeStartupRequest(
            generation = 1L,
            session = session,
            endpoint = validEndpoint,
            startAttemptId = "attempt-3"
        )
        val result = detector.awaitStartup(request)
        assertTrue("Expected Ready but was $result", result is RuntimeStartupResult.Ready)
        val ready = result as RuntimeStartupResult.Ready
        assertEquals(3, ready.probeCount)
    }

    @Test
    fun readinessStartingStatus_continuesPolling() {
        val session = createAlwaysAliveSession()
        val probe = createFakeReadinessProbe(
            listOf(
                RuntimeHealthProbeResult.Success(200, validStartingResponse()),
                RuntimeHealthProbeResult.Success(200, validStartingResponse()),
                RuntimeHealthProbeResult.Success(200, validReadyResponse())
            )
        )
        val detector = DefaultRuntimeStartupDetector(
            healthProbe = probe,
            totalStartupTimeoutMs = 5000L,
            initialProbeIntervalMs = 10L,
            maxProbeIntervalMs = 50L
        )
        val request = RuntimeStartupRequest(
            generation = 1L,
            session = session,
            endpoint = validEndpoint,
            startAttemptId = "attempt-4"
        )
        val result = detector.awaitStartup(request)
        assertTrue("Expected Ready but was $result", result is RuntimeStartupResult.Ready)
    }

    @Test
    fun degradedBackendIsAcceptedAsReady() {
        val session = createAlwaysAliveSession()
        val probe = createFakeReadinessProbe(
            listOf(
                RuntimeHealthProbeResult.Success(200, validDegradedResponse())
            )
        )
        val detector = DefaultRuntimeStartupDetector(
            healthProbe = probe,
            totalStartupTimeoutMs = 5000L,
            initialProbeIntervalMs = 10L,
            maxProbeIntervalMs = 50L
        )
        val request = RuntimeStartupRequest(
            generation = 1L,
            session = session,
            endpoint = validEndpoint,
            startAttemptId = "attempt-5"
        )
        val result = detector.awaitStartup(request)
        assertTrue("Expected Ready but was $result", result is RuntimeStartupResult.Ready)
    }

    @Test
    fun prootExitedBeforeProbe_returnsFailedWithProotExited() {
        val endpoint = validEndpoint
        val session = createDeadSession(exitCode = 137)
        val probe = createFakeReadinessProbe(listOf(RuntimeHealthProbeResult.Success(200, validReadyResponse())))
        val detector = DefaultRuntimeStartupDetector(
            healthProbe = probe,
            totalStartupTimeoutMs = 5000L,
            initialProbeIntervalMs = 10L,
            maxProbeIntervalMs = 50L
        )
        val request = RuntimeStartupRequest(
            generation = 1L,
            session = session,
            endpoint = endpoint,
            startAttemptId = "attempt-6"
        )
        val result = detector.awaitStartup(request)
        assertTrue("Expected Failed but was $result", result is RuntimeStartupResult.Failed)
        val failed = result as RuntimeStartupResult.Failed
        assertTrue(
            "Expected ProotExited but was ${failed.error}",
            failed.error is RuntimeStartupError.ProotExited
        )
        val exited = failed.error as RuntimeStartupError.ProotExited
        assertEquals(137, exited.exitCode)
    }

    @Test
    fun prootExitedDuringPolling_returnsFailedWithProotExited_notTimeout() {
        val session = createLiveThenDeadSession(aliveForProbes = 2)
        val probe = createFakeReadinessProbe(
            listOf(
                RuntimeHealthProbeResult.Failure(RuntimeHealthProbeError.ConnectionRefused),
                RuntimeHealthProbeResult.Failure(RuntimeHealthProbeError.ConnectionRefused),
                RuntimeHealthProbeResult.Success(503, blocked503Response())
            )
        )
        val detector = DefaultRuntimeStartupDetector(
            healthProbe = probe,
            totalStartupTimeoutMs = 30_000L,
            initialProbeIntervalMs = 10L,
            maxProbeIntervalMs = 50L
        )
        val request = RuntimeStartupRequest(
            generation = 1L,
            session = session,
            endpoint = validEndpoint,
            startAttemptId = "attempt-7"
        )
        val result = detector.awaitStartup(request)
        assertTrue("Expected Failed but was $result", result is RuntimeStartupResult.Failed)
        val failed = result as RuntimeStartupResult.Failed
        assertFalse("Timeout would hide root cause. Was $failed.error", failed.error is RuntimeStartupError.Timeout)
        assertTrue(
            "Expected ProotExited but was ${failed.error}",
            failed.error is RuntimeStartupError.ProotExited
        )
    }

    @Test
    fun overallTimeout_returnsFailedWithTimeout() {
        val session = createAlwaysAliveSession()
        val probe = createFakeReadinessProbe(
            listOf(RuntimeHealthProbeResult.Failure(RuntimeHealthProbeError.ConnectionRefused))
        )
        val detector = DefaultRuntimeStartupDetector(
            healthProbe = probe,
            totalStartupTimeoutMs = 300L,
            initialProbeIntervalMs = 30L,
            maxProbeIntervalMs = 60L
        )
        val request = RuntimeStartupRequest(
            generation = 1L,
            session = session,
            endpoint = validEndpoint,
            startAttemptId = "attempt-8"
        )
        val result = detector.awaitStartup(request)
        assertTrue("Expected Failed but was $result", result is RuntimeStartupResult.Failed)
        val failed = result as RuntimeStartupResult.Failed
        assertTrue("Expected Timeout but was ${failed.error}", failed.error is RuntimeStartupError.Timeout)
        val timeout = failed.error as RuntimeStartupError.Timeout
        assertTrue(timeout.probeCount > 0)
    }

    @Test
    fun cancelDuringBackoff_returnsCancelled() {
        val session = createAlwaysAliveSession()
        val probe = createFakeReadinessProbe(
            listOf(RuntimeHealthProbeResult.Failure(RuntimeHealthProbeError.ConnectionRefused))
        )
        val detector = DefaultRuntimeStartupDetector(
            healthProbe = probe,
            totalStartupTimeoutMs = 30_000L,
            initialProbeIntervalMs = 200L,
            maxProbeIntervalMs = 500L
        )
        val request = RuntimeStartupRequest(
            generation = 1L,
            session = session,
            endpoint = validEndpoint,
            startAttemptId = "attempt-9"
        )
        val canceller = Thread {
            Thread.sleep(50)
            detector.cancel()
        }.apply { isDaemon = true; start() }
        val result = detector.awaitStartup(request)
        assertTrue("Expected Cancelled but was $result", result is RuntimeStartupResult.Cancelled)
    }

    @Test
    fun readiness401_returnsProtocolFailure() {
        val session = createAlwaysAliveSession()
        val probe = createFakeReadinessProbe(
            listOf(RuntimeHealthProbeResult.Success(401, """{"error":"unauthorized"}"""))
        )
        val detector = DefaultRuntimeStartupDetector(
            healthProbe = probe,
            totalStartupTimeoutMs = 5000L,
            initialProbeIntervalMs = 10L,
            maxProbeIntervalMs = 50L
        )
        val request = RuntimeStartupRequest(
            generation = 1L,
            session = session,
            endpoint = validEndpoint,
            startAttemptId = "attempt-10"
        )
        val result = detector.awaitStartup(request)
        assertTrue("Expected Failed but was $result", result is RuntimeStartupResult.Failed)
        val failed = result as RuntimeStartupResult.Failed
        assertTrue(
            "Expected InvalidResponse but was ${failed.error}",
            failed.error is RuntimeStartupError.InvalidResponse
        )
    }

    @Test
    fun readiness404_returnsProtocolFailure() {
        val session = createAlwaysAliveSession()
        val probe = createFakeReadinessProbe(
            listOf(RuntimeHealthProbeResult.Success(404, ""))
        )
        val detector = DefaultRuntimeStartupDetector(
            healthProbe = probe,
            totalStartupTimeoutMs = 5000L,
            initialProbeIntervalMs = 10L,
            maxProbeIntervalMs = 50L
        )
        val request = RuntimeStartupRequest(
            generation = 1L,
            session = session,
            endpoint = validEndpoint,
            startAttemptId = "attempt-11"
        )
        val result = detector.awaitStartup(request)
        assertTrue("Expected Failed but was $result", result is RuntimeStartupResult.Failed)
        val failed = result as RuntimeStartupResult.Failed
        assertTrue(
            "Expected InvalidResponse but was ${failed.error}",
            failed.error is RuntimeStartupError.InvalidResponse
        )
    }

    @Test
    fun invalidEndpoint_returnsInvalidEndpoint() {
        val session = createAlwaysAliveSession()
        val invalidPolicy = BackendEndpointPolicy(
            host = "192.168.1.100",
            port = 18899,
            httpScheme = "http",
            webSocketScheme = "ws"
        )
        val probe = createFakeReadinessProbe(emptyList())
        val detector = DefaultRuntimeStartupDetector(healthProbe = probe, clock = SystemRuntimeClock)
        val request = RuntimeStartupRequest(
            generation = 1L,
            session = session,
            endpoint = invalidPolicy,
            startAttemptId = "attempt-12"
        )
        val result = detector.awaitStartup(request)
        assertTrue("Expected Failed but was $result", result is RuntimeStartupResult.Failed)
        val failed = result as RuntimeStartupResult.Failed
        assertEquals(RuntimeStartupError.InvalidEndpoint, failed.error)
    }

    @Test
    fun missingStartAttemptId_rejectedByRequire() {
        val session = createAlwaysAliveSession()
        val probe = createFakeReadinessProbe(emptyList())
        val detector = DefaultRuntimeStartupDetector(
            healthProbe = probe,
            totalStartupTimeoutMs = 100L,
            initialProbeIntervalMs = 10L
        )
        val request = RuntimeStartupRequest(
            generation = 1L,
            session = session,
            endpoint = validEndpoint,
            startAttemptId = ""
        )
        try {
            detector.awaitStartup(request)
            throw AssertionError("Expected IllegalArgumentException")
        } catch (_: IllegalArgumentException) {
        }
    }

    @Test
    fun duplicateDetectorInvocation_rejected() {
        val session = createAlwaysAliveSession()
        val probe = createFakeReadinessProbe(
            listOf(RuntimeHealthProbeResult.Failure(RuntimeHealthProbeError.ConnectionRefused))
        )
        val detector = DefaultRuntimeStartupDetector(
            healthProbe = probe,
            totalStartupTimeoutMs = 5000L,
            initialProbeIntervalMs = 10L
        )
        val request1 = RuntimeStartupRequest(
            generation = 1L,
            session = session,
            endpoint = validEndpoint,
            startAttemptId = "attempt-13"
        )
        val request2 = RuntimeStartupRequest(
            generation = 1L,
            session = session,
            endpoint = validEndpoint,
            startAttemptId = "attempt-13b"
        )
        val resultHolder = AtomicReference<RuntimeStartupResult?>(null)
        val t = Thread {
            resultHolder.set(detector.awaitStartup(request1))
        }.apply { isDaemon = true; start() }
        Thread.sleep(30)
        val duplicateResult = detector.awaitStartup(request2)
        t.join(5000)
        assertTrue(
            "Expected Failed (duplicate) but was $duplicateResult",
            duplicateResult is RuntimeStartupResult.Failed
        )
        val failed = duplicateResult as RuntimeStartupResult.Failed
        assertTrue(failed.error is RuntimeStartupError.InternalError)
    }

    @Test
    fun noBackendServices_onForeignPort_returnsInvalidResponse_fastNotReady() {
        val session = createAlwaysAliveSession()
        val foreignServiceResponse = """{"status":"200","content":"not-amitia-backend"}"""
        val probe = createFakeReadinessProbe(
            listOf(RuntimeHealthProbeResult.Success(200, foreignServiceResponse))
        )
        val detector = DefaultRuntimeStartupDetector(
            healthProbe = probe,
            totalStartupTimeoutMs = 5000L,
            initialProbeIntervalMs = 10L,
            maxProbeIntervalMs = 50L
        )
        val request = RuntimeStartupRequest(
            generation = 1L,
            session = session,
            endpoint = validEndpoint,
            startAttemptId = "attempt-foreign-200"
        )
        val result = detector.awaitStartup(request)
        assertTrue(
            "Expected Failed (InvalidResponse for foreign HTTP service) but was $result",
            result is RuntimeStartupResult.Failed
        )
        val failed = result as RuntimeStartupResult.Failed
        assertTrue(
            "Expected InvalidResponse but was ${failed.error}",
            failed.error is RuntimeStartupError.InvalidResponse
        )
    }

    @Test
    fun malformedResponse_returnsInvalidResponse() {
        val session = createAlwaysAliveSession()
        val probe = createFakeReadinessProbe(
            listOf(
                RuntimeHealthProbeResult.Success(200, "this-is-not-json"),
                RuntimeHealthProbeResult.Success(200, """{"code":200,"data":{}}""")
            )
        )
        val detector = DefaultRuntimeStartupDetector(
            healthProbe = probe,
            totalStartupTimeoutMs = 5000L,
            initialProbeIntervalMs = 10L,
            maxProbeIntervalMs = 50L
        )
        val request = RuntimeStartupRequest(
            generation = 1L,
            session = session,
            endpoint = validEndpoint,
            startAttemptId = "attempt-malformed"
        )
        val result = detector.awaitStartup(request)
        assertTrue(
            "Expected Failed for malformed body but was $result",
            result is RuntimeStartupResult.Failed
        )
    }
}
