package com.amitia.amitia_app.runtime.startup

import com.amitia.amitia_app.runtime.connection.BackendEndpointPolicy
import com.amitia.amitia_app.runtime.internal.RuntimeClock
import com.amitia.amitia_app.runtime.internal.SystemRuntimeClock
import com.amitia.amitia_app.runtime.proot.ProotSession
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicReference

internal data class RuntimeStartupRequest(
    val generation: Long,
    val session: ProotSession,
    val endpoint: BackendEndpointPolicy,
    val startAttemptId: String
)

internal interface RuntimeStartupDetector {
    fun awaitStartup(request: RuntimeStartupRequest): RuntimeStartupResult
    fun cancel()
}

internal class DefaultRuntimeStartupDetector(
    private val healthProbe: RuntimeHealthProbe = HttpRuntimeHealthProbe(),
    private val clock: RuntimeClock = SystemRuntimeClock,
    private val totalStartupTimeoutMs: Long = DEFAULT_STARTUP_TIMEOUT_MS,
    private val initialProbeIntervalMs: Long = DEFAULT_INITIAL_PROBE_INTERVAL_MS,
    private val maxProbeIntervalMs: Long = DEFAULT_MAX_PROBE_INTERVAL_MS,
    private val probeBackoffMultiplier: Int = DEFAULT_PROBE_BACKOFF_MULTIPLIER,
    private val maxProbeCount: Int = DEFAULT_MAX_PROBE_COUNT
) : RuntimeStartupDetector {

    private val cancelledFlag = AtomicBoolean(false)
    private val activeRequest = AtomicReference<RuntimeStartupRequest?>(null)
    private val workerThread = AtomicReference<Thread?>(null)

    override fun awaitStartup(request: RuntimeStartupRequest): RuntimeStartupResult {
        require(request.generation >= 0) { "generation must be non-negative" }
        require(request.startAttemptId.isNotBlank()) { "startAttemptId must not be blank" }
        require(request.endpoint.port in 1..65535) { "invalid port: ${request.endpoint.port}" }

        if (!HttpRuntimeHealthProbe.isLoopbackHost(request.endpoint.host)) {
            return RuntimeStartupResult.Failed(
                request.generation,
                RuntimeStartupError.InvalidEndpoint
            )
        }

        cancelledFlag.set(false)

        if (!activeRequest.compareAndSet(null, request)) {
            return RuntimeStartupResult.Failed(
                request.generation,
                RuntimeStartupError.InternalError("another startup detection is already active")
            )
        }

        try {
            return executeDetectionLoop(request)
        } finally {
            activeRequest.set(null)
        }
    }

    private fun executeDetectionLoop(request: RuntimeStartupRequest): RuntimeStartupResult {
        val startEpochMs = clock.nowEpochMillis()
        val deadlineEpochMs = startEpochMs + totalStartupTimeoutMs
        var probeIntervalMs = initialProbeIntervalMs
        var probeCount = 0

        while (true) {
            if (cancelledFlag.get()) {
                return RuntimeStartupResult.Cancelled(request.generation)
            }

            val now = clock.nowEpochMillis()
            if (now >= deadlineEpochMs || probeCount >= maxProbeCount) {
                val elapsed = now - startEpochMs
                return RuntimeStartupResult.Failed(
                    request.generation,
                    RuntimeStartupError.Timeout(totalStartupTimeoutMs, elapsed, probeCount)
                )
            }

            if (!request.session.isAlive()) {
                val exitCode = request.session.awaitExit(0)
                val elapsed = clock.nowEpochMillis() - startEpochMs
                return RuntimeStartupResult.Failed(
                    request.generation,
                    RuntimeStartupError.ProotExited(exitCode, elapsed)
                )
            }

            val probeResult = runSingleProbe(request, probeCount, startEpochMs)
            probeCount++

            when (probeResult) {
                is SingleProbeOutcome.Ready -> {
                    val elapsed = clock.nowEpochMillis() - startEpochMs
                    return RuntimeStartupResult.Ready(request.generation, elapsed, probeCount)
                }
                is SingleProbeOutcome.Continue -> {
                }
                is SingleProbeOutcome.ProtocolFailure -> {
                    val elapsed = clock.nowEpochMillis() - startEpochMs
                    return RuntimeStartupResult.Failed(
                        request.generation,
                        RuntimeStartupError.InvalidResponse(probeResult.reason, elapsed)
                    )
                }
                is SingleProbeOutcome.Fatal -> {
                    val elapsed = clock.nowEpochMillis() - startEpochMs
                    return RuntimeStartupResult.Failed(request.generation, probeResult.error)
                }
            }

            val sleepTargetMs = clock.nowEpochMillis() + probeIntervalMs
            while (clock.nowEpochMillis() < sleepTargetMs) {
                if (cancelledFlag.get()) {
                    return RuntimeStartupResult.Cancelled(request.generation)
                }
                if (!request.session.isAlive()) break
                try {
                    Thread.sleep(minOf(POLL_GRANULARITY_MS, sleepTargetMs - clock.nowEpochMillis()))
                } catch (_: InterruptedException) {
                    Thread.currentThread().interrupt()
                    return RuntimeStartupResult.Cancelled(request.generation)
                }
            }

            probeIntervalMs = minOf(probeIntervalMs * probeBackoffMultiplier, maxProbeIntervalMs)
        }
    }

    private sealed class SingleProbeOutcome {
        data object Ready : SingleProbeOutcome()
        data object Continue : SingleProbeOutcome()
        data class ProtocolFailure(val reason: String) : SingleProbeOutcome()
        data class Fatal(val error: RuntimeStartupError) : SingleProbeOutcome()
    }

    private fun runSingleProbe(request: RuntimeStartupRequest, probeIndex: Int, startEpochMs: Long): SingleProbeOutcome {
        val result = healthProbe.checkReadiness(request.endpoint)
        when (result) {
            is RuntimeHealthProbeResult.Success -> {
                return interpretReadinessResponse(result, request, startEpochMs)
            }
            is RuntimeHealthProbeResult.Failure -> {
                return when (result.error) {
                    is RuntimeHealthProbeError.ConnectionRefused -> SingleProbeOutcome.Continue
                    is RuntimeHealthProbeError.ConnectionTimeout -> SingleProbeOutcome.Continue
                    is RuntimeHealthProbeError.Unauthorized -> SingleProbeOutcome.ProtocolFailure("readiness probe returned 401")
                    is RuntimeHealthProbeError.Forbidden -> SingleProbeOutcome.ProtocolFailure("readiness probe returned 403")
                    is RuntimeHealthProbeError.NotFound -> SingleProbeOutcome.ProtocolFailure("readiness endpoint returned 404")
                    is RuntimeHealthProbeError.ServerError -> SingleProbeOutcome.Continue
                    is RuntimeHealthProbeError.IOError -> SingleProbeOutcome.Continue
                    is RuntimeHealthProbeError.MalformedResponse -> SingleProbeOutcome.ProtocolFailure("readiness probe io error")
                }
            }
        }
    }

    private fun interpretReadinessResponse(
        result: RuntimeHealthProbeResult.Success,
        request: RuntimeStartupRequest,
        startEpochMs: Long
    ): SingleProbeOutcome {
        return when (result.statusCode) {
            200 -> {
                val status = HttpRuntimeHealthProbe.readBackendStatus(result.body)
                if (status == "ready") {
                    if (!request.session.isAlive()) {
                        SingleProbeOutcome.Fatal(
                            RuntimeStartupError.ProotExited(request.session.awaitExit(0), clock.nowEpochMillis() - startEpochMs)
                        )
                    } else {
                        SingleProbeOutcome.Ready
                    }
                } else if (status == "degraded") {
                    if (!request.session.isAlive()) {
                        SingleProbeOutcome.Fatal(
                            RuntimeStartupError.ProotExited(request.session.awaitExit(0), clock.nowEpochMillis() - startEpochMs)
                        )
                    } else {
                        SingleProbeOutcome.Ready
                    }
                } else if (status == "starting") {
                    SingleProbeOutcome.Continue
                } else {
                    SingleProbeOutcome.ProtocolFailure("readiness endpoint returned unexpected status: $status")
                }
            }
            503 -> {
                SingleProbeOutcome.Continue
            }
            in 500..599 -> {
                SingleProbeOutcome.Continue
            }
            401, 403 -> {
                SingleProbeOutcome.ProtocolFailure("readness probe returned ${result.statusCode}")
            }
            404 -> {
                SingleProbeOutcome.ProtocolFailure("readiness endpoint returned 404")
            }
            301, 302, 303, 307, 308 -> {
                SingleProbeOutcome.ProtocolFailure("readiness probe rejected: redirect ${result.statusCode}")
            }
            else -> {
                if (result.statusCode in 400..499) {
                    SingleProbeOutcome.ProtocolFailure("readness probe returned unsupported ${result.statusCode}")
                } else {
                    SingleProbeOutcome.Continue
                }
            }
        }
    }

    override fun cancel() {
        cancelledFlag.set(true)
        activeRequest.set(null)
        workerThread.getAndSet(null)?.interrupt()
    }

    private companion object {
        const val POLL_GRANULARITY_MS = 50L
        const val DEFAULT_STARTUP_TIMEOUT_MS = 90_000L
        const val DEFAULT_INITIAL_PROBE_INTERVAL_MS = 100L
        const val DEFAULT_MAX_PROBE_INTERVAL_MS = 1_000L
        const val DEFAULT_PROBE_BACKOFF_MULTIPLIER = 2
        const val DEFAULT_MAX_PROBE_COUNT = 600

        fun minOf(a: Long, b: Long): Long = if (a < b) a else b
    }
}
