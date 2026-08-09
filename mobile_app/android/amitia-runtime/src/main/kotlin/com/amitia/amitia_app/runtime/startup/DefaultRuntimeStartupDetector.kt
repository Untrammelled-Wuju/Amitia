package com.amitia.amitia_app.runtime.startup

import java.util.concurrent.atomic.AtomicBoolean

internal class DefaultRuntimeStartupDetector(
    private val healthProbe: RuntimeHealthProbe = HttpRuntimeHealthProbe(),
    private val totalTimeoutMs: Long = 60_000L,
    private val initialPollIntervalMs: Long = 250L,
    private val maxPollIntervalMs: Long = 1_000L
) : RuntimeStartupDetector {

    private val cancelled = AtomicBoolean(false)

    override fun awaitStartup(request: RuntimeStartupRequest): RuntimeStartupResult {
        cancelled.set(false)

        val deadline = System.currentTimeMillis() + totalTimeoutMs
        var currentPhase = RuntimeStartupPhase.WAITING_FOR_PROOT
        var pollInterval = initialPollIntervalMs

        try {
            while (true) {
                if (cancelled.get()) {
                    return RuntimeStartupResult.Cancelled(request.generation)
                }

                if (System.currentTimeMillis() >= deadline) {
                    return RuntimeStartupResult.Failed(request.generation, RuntimeStartupError.Timeout)
                }

                if (!request.session.isAlive()) {
                    val exitCode = request.session.awaitExit(100) ?: -1
                    return RuntimeStartupResult.Failed(
                        request.generation,
                        RuntimeStartupError.ProotExited(exitCode)
                    )
                }

                when (currentPhase) {
                    RuntimeStartupPhase.WAITING_FOR_PROOT -> {
                        val livenessResult = healthProbe.checkLiveness(request.endpoint)
                        if (livenessResult is RuntimeHealthProbeResult.Success) {
                            if (livenessResult.statusCode == 200) {
                                currentPhase = RuntimeStartupPhase.WAITING_FOR_BACKEND_READY
                            } else if (livenessResult.statusCode == 401 || livenessResult.statusCode == 403) {
                                return RuntimeStartupResult.Failed(
                                    request.generation,
                                    RuntimeStartupError.HealthAuthFailed
                                )
                            } else if (livenessResult.statusCode == 404) {
                                return RuntimeStartupResult.Failed(
                                    request.generation,
                                    RuntimeStartupError.HealthEndpointMissing
                                )
                            }
                        }
                    }

                    RuntimeStartupPhase.WAITING_FOR_BACKEND_READY -> {
                        val readinessResult = healthProbe.checkReadiness(request.endpoint)
                        when {
                            readinessResult is RuntimeHealthProbeResult.Success && readinessResult.statusCode == 200 -> {
                                return RuntimeStartupResult.Ready(request.generation)
                            }
                            readinessResult is RuntimeHealthProbeResult.Success && readinessResult.statusCode == 503 -> {
                            }
                            readinessResult is RuntimeHealthProbeResult.Success && readinessResult.statusCode == 401 -> {
                                return RuntimeStartupResult.Failed(
                                    request.generation,
                                    RuntimeStartupError.HealthAuthFailed
                                )
                            }
                            readinessResult is RuntimeHealthProbeResult.Success && readinessResult.statusCode == 403 -> {
                                return RuntimeStartupResult.Failed(
                                    request.generation,
                                    RuntimeStartupError.HealthAuthFailed
                                )
                            }
                            readinessResult is RuntimeHealthProbeResult.Success && readinessResult.statusCode == 404 -> {
                                return RuntimeStartupResult.Failed(
                                    request.generation,
                                    RuntimeStartupError.HealthEndpointMissing
                                )
                            }
                            readinessResult is RuntimeHealthProbeResult.Failure &&
                                readinessResult.error is RuntimeHealthProbeError.ConnectionRefused -> {
                            }
                        }
                    }

                    else -> {}
                }

                val remainingTime = deadline - System.currentTimeMillis()
                if (remainingTime <= 0) {
                    return RuntimeStartupResult.Failed(request.generation, RuntimeStartupError.Timeout)
                }

                val sleepTime = minOf(pollInterval, remainingTime)
                Thread.sleep(sleepTime)

                pollInterval = minOf(pollInterval + 250, maxPollIntervalMs)
            }
        } catch (e: InterruptedException) {
            Thread.currentThread().interrupt()
            return RuntimeStartupResult.Cancelled(request.generation)
        }
    }

    override fun cancel() {
        cancelled.set(true)
    }
}

private fun minOf(a: Long, b: Long): Long = if (a < b) a else b
